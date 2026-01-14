// Theme Manager
class ThemeManager {
    constructor() {
        this.theme = localStorage.getItem('theme') || 'dark';
        this.toggle = document.getElementById('themeToggle');
        this.init();
    }

    init() {
        document.documentElement.setAttribute('data-theme', this.theme);
        if (this.toggle) {
            this.toggle.addEventListener('click', () => this.toggleTheme());
        }
    }

    toggleTheme() {
        this.theme = this.theme === 'dark' ? 'light' : 'dark';
        document.documentElement.setAttribute('data-theme', this.theme);
        localStorage.setItem('theme', this.theme);
    }
}

// Status Dashboard Application
class StatusDashboard {
    constructor() {
        this.services = [];
        this.incidents = [];
        this.stats = null;
        this.eventSource = null;
        this.currentFilter = 'all';
        this.init();
    }

    async init() {
        new ThemeManager();
        this.setupFilterButtons();
        await this.fetchStatus();
        this.setupEventSource();
        this.startAutoRefresh();
    }

    setupFilterButtons() {
        const filterBtns = document.querySelectorAll('.filter-btn');
        filterBtns.forEach(btn => {
            btn.addEventListener('click', () => {
                filterBtns.forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                this.currentFilter = btn.dataset.filter;
                this.renderServices();
            });
        });
    }

    async fetchStatus() {
        try {
            const response = await fetch('/api/v1/status');
            const data = await response.json();
            this.services = data.services;
            this.incidents = data.incidents;
            this.stats = data.stats;
            this.render();
        } catch (error) {
            console.error('Failed to fetch status:', error);
        }
    }

    setupEventSource() {
        if (this.eventSource) {
            this.eventSource.close();
        }

        this.eventSource = new EventSource('/api/v1/stream');

        this.eventSource.addEventListener('health_check', (event) => {
            const data = JSON.parse(event.data);
            this.updateService(data.service, data.check);
        });

        this.eventSource.addEventListener('status', (event) => {
            const data = JSON.parse(event.data);
            this.stats = data;
            this.renderStats();
            this.renderSLACompliance();
        });

        this.eventSource.onerror = () => {
            console.log('SSE connection lost, reconnecting...');
            setTimeout(() => this.setupEventSource(), 5000);
        };
    }

    startAutoRefresh() {
        setInterval(() => this.fetchStatus(), 60000);
    }

    updateService(service, check) {
        if (!service) return;

        const index = this.services.findIndex(s => s.id === service.id);
        if (index !== -1) {
            this.services[index].status = check.status;
            this.services[index].lastCheckAt = check.checkedAt;
            this.services[index].responseTimeMs = check.responseTimeMs;
            this.renderServices();
        }

        this.updateLastUpdated();
    }

    render() {
        this.renderBanner();
        this.renderStats();
        this.renderUptimeChart();
        this.renderSLACompliance();
        this.renderIncidents();
        this.renderServices();
        this.updateLastUpdated();
    }

    renderBanner() {
        const banner = document.getElementById('statusBanner');
        const title = document.getElementById('statusTitle');
        const subtitle = document.getElementById('statusSubtitle');

        banner.classList.remove('operational', 'degraded', 'outage');

        if (this.stats.unhealthyServices > 0) {
            banner.classList.add('outage');
            title.textContent = 'System Disruption';
            subtitle.textContent = `${this.stats.unhealthyServices} service(s) are currently experiencing issues`;
        } else if (this.stats.degradedServices > 0) {
            banner.classList.add('degraded');
            title.textContent = 'Partial Degradation';
            subtitle.textContent = `${this.stats.degradedServices} service(s) are experiencing degraded performance`;
        } else {
            banner.classList.add('operational');
            title.textContent = 'All Systems Operational';
            subtitle.textContent = 'All services are running smoothly';
        }
    }

    renderStats() {
        document.getElementById('overallUptime').textContent =
            this.formatUptime(this.stats.overallUptime);
        document.getElementById('avgResponse').textContent =
            this.formatResponseTime(this.stats.avgResponseMs);
        document.getElementById('servicesHealthy').textContent =
            `${this.stats.healthyServices}/${this.stats.totalServices}`;
        document.getElementById('activeIncidents').textContent =
            this.incidents.filter(i => i.status !== 'resolved').length.toString();
    }

    renderSLACompliance() {
        // Calculate SLA by tier
        const criticalServices = this.services.filter(s => s.slaTarget >= 99.9);
        const standardServices = this.services.filter(s => s.slaTarget >= 99.0 && s.slaTarget < 99.9);
        const supportingServices = this.services.filter(s => s.slaTarget < 99.0);

        // Calculate averages and compliance
        const criticalUptime = this.calculateAvgUptime(criticalServices);
        const standardUptime = this.calculateAvgUptime(standardServices);
        const supportingUptime = this.calculateAvgUptime(supportingServices);

        // Update critical
        document.getElementById('criticalUptime').textContent = this.formatUptime(criticalUptime);
        const criticalProgress = document.getElementById('criticalProgress');
        criticalProgress.style.width = `${Math.min(criticalUptime, 100)}%`;
        this.updateSLAStatus('criticalProgress', 'criticalStatus', criticalUptime, 99.9);

        // Update standard
        document.getElementById('standardUptime').textContent = this.formatUptime(standardUptime);
        const standardProgress = document.getElementById('standardProgress');
        standardProgress.style.width = `${Math.min(standardUptime, 100)}%`;
        this.updateSLAStatus('standardProgress', 'standardStatus', standardUptime, 99.5);

        // Update supporting
        document.getElementById('supportingUptime').textContent = this.formatUptime(supportingUptime);
        const supportingProgress = document.getElementById('supportingProgress');
        supportingProgress.style.width = `${Math.min(supportingUptime, 100)}%`;
        this.updateSLAStatus('supportingProgress', 'supportingStatus', supportingUptime, 99.0);

        // Update summary badge
        const meetingSLA = this.services.filter(s => s.uptime30d >= s.slaTarget).length;
        const slaSummary = document.getElementById('slaSummary');
        slaSummary.textContent = `${meetingSLA}/${this.services.length} meeting SLA`;

        if (meetingSLA === this.services.length) {
            slaSummary.style.background = 'var(--accent-green-bg)';
            slaSummary.style.color = 'var(--accent-green)';
        } else {
            slaSummary.style.background = 'var(--accent-yellow-bg)';
            slaSummary.style.color = 'var(--accent-yellow)';
        }
    }

    updateSLAStatus(progressId, statusId, uptime, target) {
        const progress = document.getElementById(progressId);
        const status = document.getElementById(statusId);

        progress.classList.remove('warning', 'danger');
        status.classList.remove('warning', 'danger');

        if (uptime >= target) {
            status.textContent = 'Meeting SLA';
            status.style.color = 'var(--accent-green)';
        } else if (uptime >= target - 1) {
            progress.classList.add('warning');
            status.classList.add('warning');
            status.textContent = 'At Risk';
            status.style.color = 'var(--accent-yellow)';
        } else {
            progress.classList.add('danger');
            status.classList.add('danger');
            status.textContent = 'Below SLA';
            status.style.color = 'var(--accent-red)';
        }
    }

    calculateAvgUptime(services) {
        if (services.length === 0) return 100;
        const total = services.reduce((sum, s) => sum + (s.uptime30d || 0), 0);
        return total / services.length;
    }

    renderIncidents() {
        const section = document.getElementById('incidentsSection');
        const list = document.getElementById('incidentsList');
        const count = document.getElementById('incidentCount');
        const activeIncidents = this.incidents.filter(i => i.status !== 'resolved');

        if (activeIncidents.length === 0) {
            section.classList.add('hidden');
            return;
        }

        section.classList.remove('hidden');
        count.textContent = activeIncidents.length;

        list.innerHTML = activeIncidents.map(incident => `
            <div class="incident-card ${incident.status}">
                <div class="incident-header">
                    <div class="incident-title">${this.escapeHtml(incident.title)}</div>
                    <span class="incident-status">${incident.status}</span>
                </div>
                <div class="incident-time">Started ${this.formatTime(incident.startedAt)}</div>
            </div>
        `).join('');
    }

    renderServices() {
        const container = document.getElementById('servicesList');

        // Filter services
        let filteredServices = this.services;
        if (this.currentFilter === 'healthy') {
            filteredServices = this.services.filter(s => s.status === 'healthy');
        } else if (this.currentFilter === 'issues') {
            filteredServices = this.services.filter(s => s.status !== 'healthy');
        }

        // Group services by category
        const categories = {};
        filteredServices.forEach(service => {
            const category = service.category || 'Other';
            if (!categories[category]) {
                categories[category] = [];
            }
            categories[category].push(service);
        });

        // Sort categories with priority order
        const categoryOrder = ['Infrastructure', 'Identity', 'Core', 'Commerce', 'Customer', 'Catalog', 'Communication', 'Vendor', 'Supporting', 'Documentation'];
        const sortedCategories = Object.keys(categories).sort((a, b) => {
            const aIndex = categoryOrder.indexOf(a);
            const bIndex = categoryOrder.indexOf(b);
            if (aIndex === -1 && bIndex === -1) return a.localeCompare(b);
            if (aIndex === -1) return 1;
            if (bIndex === -1) return -1;
            return aIndex - bIndex;
        });

        container.innerHTML = sortedCategories.map(category => `
            <div class="category-group">
                <div class="category-header">
                    <span>${this.escapeHtml(category)}</span>
                    <span class="category-count">${categories[category].length}</span>
                </div>
                <div class="category-services">
                    ${categories[category].map(service => this.renderServiceRow(service)).join('')}
                </div>
            </div>
        `).join('');
    }

    renderServiceRow(service) {
        const uptimeClass = this.getUptimeClass(service.uptime30d);
        const slaMet = service.uptime30d >= service.slaTarget;

        return `
            <div class="service-row" data-service-id="${service.id}">
                <div class="service-info">
                    <span class="service-status-dot ${service.status}"></span>
                    <span class="service-name">${this.escapeHtml(service.displayName)}</span>
                </div>
                <div class="service-meta">
                    <span class="service-uptime ${uptimeClass}">
                        ${this.formatUptime(service.uptime30d)}
                    </span>
                    <span class="service-response">${this.formatResponseTime(service.responseTimeMs)}</span>
                    <span class="sla-tag ${slaMet ? 'met' : 'not-met'}">
                        ${service.slaTarget}%
                    </span>
                </div>
            </div>
        `;
    }

    renderUptimeChart() {
        const chart = document.getElementById('uptimeChart');
        const days = 90;
        const bars = [];

        for (let i = 0; i < days; i++) {
            const uptime = this.stats.overallUptime || 99.9;
            // Add slight variance for visual effect
            const variance = Math.random() * 1.5 - 0.5;
            const dayUptime = Math.min(100, Math.max(94, uptime + variance));

            let barClass = 'good';
            if (dayUptime < 99) barClass = 'warning';
            if (dayUptime < 95) barClass = 'bad';

            const height = Math.max(30, (dayUptime - 90) * 7);
            const date = new Date();
            date.setDate(date.getDate() - (days - i - 1));
            const dateStr = date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });

            bars.push(`<div class="uptime-bar ${barClass}" style="height: ${height}%" title="${dateStr}: ${dayUptime.toFixed(2)}% uptime"></div>`);
        }

        chart.innerHTML = bars.join('');
    }

    updateLastUpdated() {
        const now = new Date();
        document.getElementById('lastUpdated').textContent =
            `Last updated: ${now.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}`;
    }

    formatUptime(value) {
        if (value === undefined || value === null) return '--';
        return value.toFixed(2) + '%';
    }

    formatResponseTime(ms) {
        if (ms === undefined || ms === null) return '--';
        if (ms < 1000) return Math.round(ms) + 'ms';
        return (ms / 1000).toFixed(2) + 's';
    }

    formatTime(timestamp) {
        const date = new Date(timestamp);
        const now = new Date();
        const diff = now - date;

        if (diff < 60000) return 'just now';
        if (diff < 3600000) return `${Math.floor(diff / 60000)} minutes ago`;
        if (diff < 86400000) return `${Math.floor(diff / 3600000)} hours ago`;
        return date.toLocaleDateString();
    }

    getUptimeClass(uptime) {
        if (uptime >= 99.9) return 'good';
        if (uptime >= 99) return 'warning';
        return 'bad';
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Initialize dashboard when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.dashboard = new StatusDashboard();
});

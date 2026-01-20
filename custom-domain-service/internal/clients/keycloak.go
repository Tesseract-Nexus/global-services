package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"custom-domain-service/internal/config"
	"custom-domain-service/internal/models"

	"github.com/rs/zerolog/log"
)

// KeycloakClient handles Keycloak admin operations
type KeycloakClient struct {
	cfg        *config.Config
	httpClient *http.Client
	token      string
	tokenExp   time.Time
	tokenMu    sync.RWMutex
}

// NewKeycloakClient creates a new Keycloak client
func NewKeycloakClient(cfg *config.Config) *KeycloakClient {
	return &KeycloakClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// keycloakToken represents the token response
type keycloakToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// keycloakClient represents a Keycloak client configuration
type keycloakClient struct {
	ID           string   `json:"id,omitempty"`
	ClientID     string   `json:"clientId"`
	Name         string   `json:"name,omitempty"`
	RedirectUris []string `json:"redirectUris"`
	WebOrigins   []string `json:"webOrigins"`
}

// getToken obtains or refreshes the admin token
func (k *KeycloakClient) getToken(ctx context.Context) (string, error) {
	k.tokenMu.RLock()
	if k.token != "" && time.Now().Before(k.tokenExp) {
		token := k.token
		k.tokenMu.RUnlock()
		return token, nil
	}
	k.tokenMu.RUnlock()

	k.tokenMu.Lock()
	defer k.tokenMu.Unlock()

	// Double-check after acquiring write lock
	if k.token != "" && time.Now().Before(k.tokenExp) {
		return k.token, nil
	}

	tokenURL := fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", k.cfg.Keycloak.AdminURL)

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", k.cfg.Keycloak.ClientID)
	data.Set("client_secret", k.cfg.Keycloak.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get token: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tokenResp keycloakToken
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	k.token = tokenResp.AccessToken
	// Set expiry with 30-second buffer
	k.tokenExp = time.Now().Add(time.Duration(tokenResp.ExpiresIn-30) * time.Second)

	return k.token, nil
}

// doRequest performs an authenticated request to Keycloak
func (k *KeycloakClient) doRequest(ctx context.Context, method, url string, body interface{}) (*http.Response, error) {
	token, err := k.getToken(ctx)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return k.httpClient.Do(req)
}

// AddDomainRedirectURIs adds redirect URIs for a custom domain to Keycloak clients
func (k *KeycloakClient) AddDomainRedirectURIs(ctx context.Context, domain *models.CustomDomain) error {
	// Get client ID by pattern matching
	clients, err := k.getClientsByPattern(ctx, domain.TenantSlug)
	if err != nil {
		return fmt.Errorf("failed to get clients: %w", err)
	}

	if len(clients) == 0 {
		log.Warn().Str("tenant", domain.TenantSlug).Msg("No Keycloak clients found for tenant")
		return nil
	}

	// Build redirect URIs for the custom domain
	redirectURIs := k.buildRedirectURIs(domain)
	webOrigins := k.buildWebOrigins(domain)

	for _, client := range clients {
		if err := k.updateClientURIs(ctx, client, redirectURIs, webOrigins); err != nil {
			log.Error().Err(err).Str("client", client.ClientID).Msg("Failed to update client URIs")
			return err
		}
		log.Info().Str("client", client.ClientID).Str("domain", domain.Domain).Msg("Updated Keycloak client redirect URIs")
	}

	return nil
}

// RemoveDomainRedirectURIs removes redirect URIs for a custom domain from Keycloak clients
func (k *KeycloakClient) RemoveDomainRedirectURIs(ctx context.Context, domain *models.CustomDomain) error {
	clients, err := k.getClientsByPattern(ctx, domain.TenantSlug)
	if err != nil {
		return fmt.Errorf("failed to get clients: %w", err)
	}

	if len(clients) == 0 {
		return nil
	}

	// Build redirect URIs to remove
	redirectURIsToRemove := k.buildRedirectURIs(domain)
	webOriginsToRemove := k.buildWebOrigins(domain)

	for _, client := range clients {
		if err := k.removeClientURIs(ctx, client, redirectURIsToRemove, webOriginsToRemove); err != nil {
			log.Error().Err(err).Str("client", client.ClientID).Msg("Failed to remove client URIs")
			return err
		}
		log.Info().Str("client", client.ClientID).Str("domain", domain.Domain).Msg("Removed Keycloak client redirect URIs")
	}

	return nil
}

// getClientsByPattern finds Keycloak clients matching the pattern for a tenant
func (k *KeycloakClient) getClientsByPattern(ctx context.Context, tenantSlug string) ([]keycloakClient, error) {
	// Search for clients matching the pattern
	pattern := strings.Replace(k.cfg.Keycloak.ClientPattern, "{tenant}", tenantSlug, -1)

	listURL := fmt.Sprintf("%s/admin/realms/%s/clients?clientId=%s",
		k.cfg.Keycloak.AdminURL, k.cfg.Keycloak.Realm, url.QueryEscape(pattern))

	resp, err := k.doRequest(ctx, "GET", listURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list clients: status %d, body: %s", resp.StatusCode, string(body))
	}

	var clients []keycloakClient
	if err := json.NewDecoder(resp.Body).Decode(&clients); err != nil {
		return nil, fmt.Errorf("failed to decode clients response: %w", err)
	}

	// Filter by pattern if needed
	var matched []keycloakClient
	for _, c := range clients {
		if strings.Contains(c.ClientID, tenantSlug) || c.ClientID == pattern {
			matched = append(matched, c)
		}
	}

	// If no direct match, try listing all and filtering
	if len(matched) == 0 && len(clients) > 0 {
		return clients, nil
	}

	return matched, nil
}

// updateClientURIs adds redirect URIs to a client
func (k *KeycloakClient) updateClientURIs(ctx context.Context, client keycloakClient, newURIs, newOrigins []string) error {
	// Merge URIs, avoiding duplicates
	uriSet := make(map[string]bool)
	for _, uri := range client.RedirectUris {
		uriSet[uri] = true
	}
	for _, uri := range newURIs {
		uriSet[uri] = true
	}

	originSet := make(map[string]bool)
	for _, origin := range client.WebOrigins {
		originSet[origin] = true
	}
	for _, origin := range newOrigins {
		originSet[origin] = true
	}

	// Convert back to slices
	updatedURIs := make([]string, 0, len(uriSet))
	for uri := range uriSet {
		updatedURIs = append(updatedURIs, uri)
	}

	updatedOrigins := make([]string, 0, len(originSet))
	for origin := range originSet {
		updatedOrigins = append(updatedOrigins, origin)
	}

	// Update client
	updateURL := fmt.Sprintf("%s/admin/realms/%s/clients/%s",
		k.cfg.Keycloak.AdminURL, k.cfg.Keycloak.Realm, client.ID)

	update := map[string]interface{}{
		"redirectUris": updatedURIs,
		"webOrigins":   updatedOrigins,
	}

	resp, err := k.doRequest(ctx, "PUT", updateURL, update)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update client: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// removeClientURIs removes redirect URIs from a client
func (k *KeycloakClient) removeClientURIs(ctx context.Context, client keycloakClient, urisToRemove, originsToRemove []string) error {
	// Build removal sets
	removeURIs := make(map[string]bool)
	for _, uri := range urisToRemove {
		removeURIs[uri] = true
	}

	removeOrigins := make(map[string]bool)
	for _, origin := range originsToRemove {
		removeOrigins[origin] = true
	}

	// Filter out URIs to remove
	updatedURIs := make([]string, 0)
	for _, uri := range client.RedirectUris {
		if !removeURIs[uri] {
			updatedURIs = append(updatedURIs, uri)
		}
	}

	updatedOrigins := make([]string, 0)
	for _, origin := range client.WebOrigins {
		if !removeOrigins[origin] {
			updatedOrigins = append(updatedOrigins, origin)
		}
	}

	// Update client
	updateURL := fmt.Sprintf("%s/admin/realms/%s/clients/%s",
		k.cfg.Keycloak.AdminURL, k.cfg.Keycloak.Realm, client.ID)

	update := map[string]interface{}{
		"redirectUris": updatedURIs,
		"webOrigins":   updatedOrigins,
	}

	resp, err := k.doRequest(ctx, "PUT", updateURL, update)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update client: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// buildRedirectURIs builds the redirect URIs for a domain
func (k *KeycloakClient) buildRedirectURIs(domain *models.CustomDomain) []string {
	uris := []string{
		fmt.Sprintf("https://%s/*", domain.Domain),
		fmt.Sprintf("https://%s/auth/callback", domain.Domain),
		fmt.Sprintf("https://%s/api/auth/callback/*", domain.Domain),
	}

	if domain.IncludeWWW && domain.DomainType == models.DomainTypeApex {
		uris = append(uris,
			fmt.Sprintf("https://www.%s/*", domain.Domain),
			fmt.Sprintf("https://www.%s/auth/callback", domain.Domain),
			fmt.Sprintf("https://www.%s/api/auth/callback/*", domain.Domain),
		)
	}

	return uris
}

// buildWebOrigins builds the web origins for a domain
func (k *KeycloakClient) buildWebOrigins(domain *models.CustomDomain) []string {
	origins := []string{
		fmt.Sprintf("https://%s", domain.Domain),
	}

	if domain.IncludeWWW && domain.DomainType == models.DomainTypeApex {
		origins = append(origins, fmt.Sprintf("https://www.%s", domain.Domain))
	}

	return origins
}

// VerifyKeycloakConnection checks if Keycloak is reachable
func (k *KeycloakClient) VerifyKeycloakConnection(ctx context.Context) error {
	_, err := k.getToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Keycloak: %w", err)
	}
	return nil
}

// GetClientRedirectURIs gets the current redirect URIs for a tenant's clients
func (k *KeycloakClient) GetClientRedirectURIs(ctx context.Context, tenantSlug string) ([]string, error) {
	clients, err := k.getClientsByPattern(ctx, tenantSlug)
	if err != nil {
		return nil, err
	}

	uriSet := make(map[string]bool)
	for _, client := range clients {
		for _, uri := range client.RedirectUris {
			uriSet[uri] = true
		}
	}

	uris := make([]string, 0, len(uriSet))
	for uri := range uriSet {
		uris = append(uris, uri)
	}

	return uris, nil
}

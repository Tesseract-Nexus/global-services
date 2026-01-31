package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"tenant-router-service/internal/config"
)

// Client handles Keycloak admin operations for redirect URI management
type Client struct {
	cfg        *config.KeycloakConfig
	httpClient *http.Client
	token      string
	tokenExp   time.Time
	tokenMu    sync.RWMutex
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type oidcClient struct {
	ID           string   `json:"id,omitempty"`
	ClientID     string   `json:"clientId"`
	RedirectUris []string `json:"redirectUris"`
	WebOrigins   []string `json:"webOrigins"`
}

// NewClient creates a new Keycloak client
func NewClient(cfg *config.KeycloakConfig) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AddTenantRedirectURIs adds redirect URIs for a tenant's hosts to all configured Keycloak clients
func (c *Client) AddTenantRedirectURIs(ctx context.Context, hosts []string) error {
	clients, err := c.getClients(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Keycloak clients: %w", err)
	}

	if len(clients) == 0 {
		log.Printf("[Keycloak] Warning: No clients found for IDs %v", c.cfg.ClientIDs)
		return nil
	}

	uris, origins := buildURIs(hosts)

	for _, client := range clients {
		if err := c.addURIs(ctx, client, uris, origins); err != nil {
			return fmt.Errorf("failed to update client %s: %w", client.ClientID, err)
		}
		log.Printf("[Keycloak] Added redirect URIs to client %s for hosts %v", client.ClientID, hosts)
	}

	return nil
}

// RemoveTenantRedirectURIs removes redirect URIs for a tenant's hosts from all configured Keycloak clients
func (c *Client) RemoveTenantRedirectURIs(ctx context.Context, hosts []string) error {
	clients, err := c.getClients(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Keycloak clients: %w", err)
	}

	uris, origins := buildURIs(hosts)

	for _, client := range clients {
		if err := c.removeURIs(ctx, client, uris, origins); err != nil {
			log.Printf("[Keycloak] Warning: Failed to remove URIs from client %s: %v", client.ClientID, err)
		}
	}

	return nil
}

// getToken obtains or refreshes the admin token
func (c *Client) getToken(ctx context.Context) (string, error) {
	c.tokenMu.RLock()
	if c.token != "" && time.Now().Before(c.tokenExp) {
		token := c.token
		c.tokenMu.RUnlock()
		return token, nil
	}
	c.tokenMu.RUnlock()

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}

	tokenURL := fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", c.cfg.AdminURL)

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.cfg.AdminClientID)
	data.Set("client_secret", c.cfg.AdminClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	c.token = tokenResp.AccessToken
	c.tokenExp = time.Now().Add(time.Duration(tokenResp.ExpiresIn-30) * time.Second)

	return c.token, nil
}

// doRequest performs an authenticated request
func (c *Client) doRequest(ctx context.Context, method, reqURL string, body interface{}) (*http.Response, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

// getClients fetches all configured Keycloak clients by their IDs
func (c *Client) getClients(ctx context.Context) ([]oidcClient, error) {
	var matched []oidcClient

	for _, clientID := range c.cfg.ClientIDs {
		listURL := fmt.Sprintf("%s/admin/realms/%s/clients?clientId=%s&first=0&max=1",
			c.cfg.AdminURL, c.cfg.Realm, url.QueryEscape(clientID))

		resp, err := c.doRequest(ctx, "GET", listURL, nil)
		if err != nil {
			return nil, err
		}

		var clients []oidcClient
		if err := json.NewDecoder(resp.Body).Decode(&clients); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		for _, cl := range clients {
			if cl.ClientID == clientID {
				matched = append(matched, cl)
			}
		}
	}

	return matched, nil
}

// addURIs adds redirect URIs and web origins to a client (deduplicating)
func (c *Client) addURIs(ctx context.Context, client oidcClient, newURIs, newOrigins []string) error {
	uriSet := make(map[string]bool)
	for _, u := range client.RedirectUris {
		uriSet[u] = true
	}
	for _, u := range newURIs {
		uriSet[u] = true
	}

	originSet := make(map[string]bool)
	for _, o := range client.WebOrigins {
		originSet[o] = true
	}
	for _, o := range newOrigins {
		originSet[o] = true
	}

	update := map[string]interface{}{
		"clientId":     client.ClientID,
		"redirectUris": setToSlice(uriSet),
		"webOrigins":   setToSlice(originSet),
	}

	updateURL := fmt.Sprintf("%s/admin/realms/%s/clients/%s",
		c.cfg.AdminURL, c.cfg.Realm, client.ID)

	resp, err := c.doRequest(ctx, "PUT", updateURL, update)
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

// removeURIs removes redirect URIs and web origins from a client
func (c *Client) removeURIs(ctx context.Context, client oidcClient, removeURIs, removeOrigins []string) error {
	removeURI := make(map[string]bool)
	for _, u := range removeURIs {
		removeURI[u] = true
	}
	removeOrigin := make(map[string]bool)
	for _, o := range removeOrigins {
		removeOrigin[o] = true
	}

	var filteredURIs []string
	for _, u := range client.RedirectUris {
		if !removeURI[u] {
			filteredURIs = append(filteredURIs, u)
		}
	}

	var filteredOrigins []string
	for _, o := range client.WebOrigins {
		if !removeOrigin[o] {
			filteredOrigins = append(filteredOrigins, o)
		}
	}

	update := map[string]interface{}{
		"clientId":     client.ClientID,
		"redirectUris": filteredURIs,
		"webOrigins":   filteredOrigins,
	}

	updateURL := fmt.Sprintf("%s/admin/realms/%s/clients/%s",
		c.cfg.AdminURL, c.cfg.Realm, client.ID)

	resp, err := c.doRequest(ctx, "PUT", updateURL, update)
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

// buildURIs builds redirect URIs and web origins for a list of hosts
func buildURIs(hosts []string) (uris []string, origins []string) {
	for _, host := range hosts {
		if host == "" {
			continue
		}
		uris = append(uris,
			fmt.Sprintf("https://%s/*", host),
			fmt.Sprintf("https://%s/auth/callback", host),
			fmt.Sprintf("https://%s/api/auth/callback/*", host),
		)
		origins = append(origins, fmt.Sprintf("https://%s", host))
	}
	return
}

func setToSlice(s map[string]bool) []string {
	result := make([]string, 0, len(s))
	for k := range s {
		result = append(result, k)
	}
	return result
}

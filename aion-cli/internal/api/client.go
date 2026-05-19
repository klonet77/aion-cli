package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/klonet77/aion-cli/internal/auth"
	"github.com/klonet77/aion-cli/internal/config"
)

// Client è il client HTTP per aion-api.
// Gestisce automaticamente il refresh del token.
type Client struct {
	httpClient  *http.Client
	creds       *config.Credentials
	keycloakURL string
}

// NewClient crea un Client a partire dalle credenziali salvate.
func NewClient(creds *config.Credentials, keycloakURL string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		creds:       creds,
		keycloakURL: keycloakURL,
	}
}

// Do esegue una request HTTP aggiungendo automaticamente il Bearer token.
// Se il token è scaduto, lo rinnova prima di procedere.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	token, err := c.validToken()
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}

// validToken ritorna un access token valido, rinnovandolo se necessario.
func (c *Client) validToken() (string, error) {
	ctx, err := c.creds.CurrentContextConfig()
	if err != nil {
		return "", err
	}

	// token ancora valido
	if !ctx.Tokens.NeedsRefresh() {
		return ctx.Tokens.AccessToken, nil
	}

	// refresh token scaduto → serve nuovo login
	if ctx.Tokens.RefreshExpired() {
		return "", fmt.Errorf("sessione scaduta — esegui 'aion login'")
	}

	// rinnova il token
	fmt.Println("🔄 Rinnovo token...")
	tr, err := auth.RefreshAccessToken(
		c.keycloakURL,
		ctx.Realm,
		ctx.ClientID,
		ctx.Tokens.RefreshToken,
	)
	if err != nil {
		return "", fmt.Errorf("token refresh failed: %w — esegui 'aion login'", err)
	}

	// aggiorna credenziali
	expiresAt, refreshExpiresAt := auth.TokenTimes(tr)
	ctx.Tokens.AccessToken = tr.AccessToken
	ctx.Tokens.RefreshToken = tr.RefreshToken
	ctx.Tokens.ExpiresAt = expiresAt
	ctx.Tokens.RefreshExpiresAt = refreshExpiresAt

	if err := config.SaveCredentials(c.creds); err != nil {
		// non fatale — il token è comunque valido per questa sessione
		fmt.Printf("⚠️  Impossibile salvare il token rinnovato: %v\n", err)
	}

	return tr.AccessToken, nil
}

// BaseURL ritorna il server URL del contesto corrente.
func (c *Client) BaseURL() (string, error) {
	ctx, err := c.creds.CurrentContextConfig()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(ctx.Server, "/"), nil
}

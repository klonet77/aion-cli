package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenResponse è la risposta di Keycloak al POST /token.
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
}

// ExchangeCode scambia il code PKCE con access_token + refresh_token.
func ExchangeCode(
	keycloakURL string,
	realm string,
	clientID string,
	code string,
	redirectURI string,
	verifier string,
) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		strings.TrimRight(keycloakURL, "/"), realm)

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", clientID)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("code_verifier", verifier)

	return postToken(tokenURL, data)
}

// RefreshAccessToken usa il refresh_token per ottenere un nuovo access_token.
func RefreshAccessToken(
	keycloakURL string,
	realm string,
	clientID string,
	refreshToken string,
) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		strings.TrimRight(keycloakURL, "/"), realm)

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", clientID)
	data.Set("refresh_token", refreshToken)

	return postToken(tokenURL, data)
}

// postToken esegue la chiamata POST all'endpoint /token di Keycloak.
func postToken(tokenURL string, data url.Values) (*TokenResponse, error) {
	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	return &tr, nil
}

// TokenTimes calcola expires_at e refresh_expires_at dal TokenResponse.
func TokenTimes(tr *TokenResponse) (expiresAt, refreshExpiresAt time.Time) {
	now := time.Now()
	expiresAt = now.Add(time.Duration(tr.ExpiresIn) * time.Second)
	refreshExpiresAt = now.Add(time.Duration(tr.RefreshExpiresIn) * time.Second)
	return
}

// ParseUserInfo estrae le informazioni utente dall'access token (senza verificare la firma).
// La firma è già stata verificata da Keycloak — qui leggiamo solo le claim.
type UserClaims struct {
	Subject           string   `json:"sub"`
	PreferredUsername string   `json:"preferred_username"`
	Email             string   `json:"email"`
	Name              string   `json:"name"`
	RealmRoles        []string `json:"-"`
}

func ParseUserInfo(accessToken string) (*UserClaims, error) {
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(accessToken, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	uc := &UserClaims{
		Subject:           asString(claims["sub"]),
		PreferredUsername: asString(claims["preferred_username"]),
		Email:             asString(claims["email"]),
		Name:              asString(claims["name"]),
	}

	// Estrai ruoli da realm_access.roles
	if ra, ok := claims["realm_access"].(map[string]any); ok {
		if roles, ok := ra["roles"].([]any); ok {
			for _, r := range roles {
				if role, ok := r.(string); ok {
					// filtra ruoli di sistema Keycloak
					if !isSystemRole(role) {
						uc.RealmRoles = append(uc.RealmRoles, role)
					}
				}
			}
		}
	}

	return uc, nil
}

func isSystemRole(role string) bool {
	system := map[string]bool{
		"offline_access":    true,
		"uma_authorization": true,
	}
	return system[role] || strings.HasPrefix(role, "default-roles-")
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

package auth

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultKeycloakURL = "https://argo.datainf.cloud"
	DefaultRealm       = "datainf"
	DefaultClientID    = "aion-cli"
	DefaultAPIURL      = "https://api.datainf.cloud"
)

type LoginResult struct {
	TokenResponse    *TokenResponse
	ExpiresAt        time.Time
	RefreshExpiresAt time.Time
}

func StartPKCEFlow(keycloakURL, realm, clientID string, openBrowser func(string) error) (*LoginResult, error) {
	pkce, err := NewPKCE()
	if err != nil {
		return nil, fmt.Errorf("generate PKCE: %w", err)
	}

	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("find free port: %w", err)
	}

	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)
	state := randomState()
	authURL := buildAuthURL(keycloakURL, realm, clientID, redirectURI, pkce.Challenge, state)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: mux}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		if q.Get("state") != state {
			errCh <- fmt.Errorf("state mismatch: possibile attacco CSRF")
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}

		if errParam := q.Get("error"); errParam != "" {
			errCh <- fmt.Errorf("keycloak error: %s — %s", errParam, q.Get("error_description"))
			fmt.Fprint(w, loginFailedHTML(errParam))
			return
		}

		code := q.Get("code")
		if code == "" {
			errCh <- fmt.Errorf("missing code in callback")
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}

		fmt.Fprint(w, loginSuccessHTML())
		codeCh <- code
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server: %w", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	fmt.Printf("🌐 Apertura browser per autenticazione...\n")
	fmt.Printf("   Se il browser non si apre, vai su:\n   %s\n\n", authURL)

	if err := openBrowser(authURL); err != nil {
		fmt.Printf("⚠️  Impossibile aprire il browser automaticamente.\n")
	}
	// fmt.Printf("DEBUG redirect_uri: %s\n", redirectURI)
	// fmt.Printf("DEBUG authURL: %s\n", authURL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		_ = srv.Shutdown(context.Background())
		return nil, err
	case <-ctx.Done():
		_ = srv.Shutdown(context.Background())
		return nil, fmt.Errorf("login timeout — riprova con 'aion login'")
	}

	_ = srv.Shutdown(context.Background())

	tr, err := ExchangeCode(keycloakURL, realm, clientID, code, redirectURI, pkce.Verifier)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	expiresAt, refreshExpiresAt := TokenTimes(tr)

	return &LoginResult{
		TokenResponse:    tr,
		ExpiresAt:        expiresAt,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func buildAuthURL(keycloakURL, realm, clientID, redirectURI, challenge, state string) string {
	base := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/auth",
		strings.TrimRight(keycloakURL, "/"), realm)

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid profile email")
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)

	return base + "?" + params.Encode()
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func randomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func loginSuccessHTML() string {
	return `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>aion-cli — Login completato</title>
<style>body{font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#f0f4f8;}
.box{text-align:center;padding:2rem;background:white;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.1);}
h1{color:#2d6a4f;}</style></head>
<body><div class="box">
<h1>✅ Login completato</h1>
<p>Puoi chiudere questa finestra e tornare al terminale.</p>
</div></body></html>`
}

func loginFailedHTML(errMsg string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>aion-cli — Errore</title>
<style>body{font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#f0f4f8;}
.box{text-align:center;padding:2rem;background:white;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.1);}
h1{color:#c0392b;}</style></head>
<body><div class="box">
<h1>❌ Login fallito</h1>
<p>%s</p>
<p>Torna al terminale e riprova con <code>aion login</code>.</p>
</div></body></html>`, errMsg)
}

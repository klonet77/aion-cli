package commands

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/klonet77/aion-cli/internal/auth"
	"github.com/klonet77/aion-cli/internal/config"
)

var (
	flagKeycloakURL string
	flagRealm       string
	flagClientID    string
	flagServer      string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Autentica con aion-api tramite browser",
	Long: `Apre il browser per autenticarti con Keycloak (PKCE flow).
Le credenziali vengono salvate in ~/.config/aion/credentials.yaml.`,
	RunE: runLogin,
}

func init() {
	loginCmd.Flags().StringVar(&flagKeycloakURL, "keycloak-url", auth.DefaultKeycloakURL, "URL di Keycloak")
	loginCmd.Flags().StringVar(&flagRealm, "realm", auth.DefaultRealm, "Realm Keycloak")
	loginCmd.Flags().StringVar(&flagClientID, "client-id", auth.DefaultClientID, "Client ID")
	loginCmd.Flags().StringVar(&flagServer, "server", auth.DefaultAPIURL, "URL di aion-api")
}

func runLogin(cmd *cobra.Command, args []string) error {
	fmt.Println("🔐 Avvio autenticazione aion...")

	// avvia flusso PKCE
	result, err := auth.StartPKCEFlow(flagKeycloakURL, flagRealm, flagClientID, openBrowser)
	if err != nil {
		return fmt.Errorf("login fallito: %w", err)
	}

	// estrai info utente dal token
	userInfo, err := auth.ParseUserInfo(result.TokenResponse.AccessToken)
	if err != nil {
		return fmt.Errorf("parse user info: %w", err)
	}

	// carica o crea credentials
	creds, err := config.LoadCredentials()
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}

	// aggiorna contesto corrente
	creds.Contexts[creds.CurrentContext] = &config.ContextConfig{
		Server:   flagServer,
		Realm:    flagRealm,
		ClientID: flagClientID,
		Tokens: config.TokenConfig{
			AccessToken:      result.TokenResponse.AccessToken,
			RefreshToken:     result.TokenResponse.RefreshToken,
			ExpiresAt:        result.ExpiresAt,
			RefreshExpiresAt: result.RefreshExpiresAt,
		},
		User: config.UserInfo{
			Subject:     userInfo.Subject,
			Email:       userInfo.Email,
			DisplayName: userInfo.Name,
			Roles:       userInfo.RealmRoles,
		},
	}

	if err := config.SaveCredentials(creds); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	fmt.Printf("\n✅ Login completato!\n")
	fmt.Printf("   Utente:  %s\n", userInfo.PreferredUsername)
	fmt.Printf("   Email:   %s\n", userInfo.Email)
	fmt.Printf("   Ruoli:   %v\n", userInfo.RealmRoles)
	fmt.Printf("   Server:  %s\n", flagServer)

	return nil
}

// openBrowser apre il browser in modo cross-platform.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("piattaforma non supportata: %s", runtime.GOOS)
	}
	return cmd.Start()
}

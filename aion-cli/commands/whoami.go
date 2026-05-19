package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/klonet77/aion-cli/internal/config"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Mostra l'utente autenticato corrente",
	RunE:  runWhoami,
}

func runWhoami(cmd *cobra.Command, args []string) error {
	creds, err := config.LoadCredentials()
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}

	if !creds.IsLoggedIn() {
		fmt.Println("❌ Non sei autenticato. Esegui 'aion login'.")
		return nil
	}

	ctx, err := creds.CurrentContextConfig()
	if err != nil {
		return err
	}

	// stato del token
	tokenStatus := "✅ valido"
	if ctx.Tokens.NeedsRefresh() {
		if ctx.Tokens.RefreshExpired() {
			tokenStatus = "❌ scaduto (esegui 'aion login')"
		} else {
			tokenStatus = "🔄 in scadenza (verrà rinnovato automaticamente)"
		}
	}

	// tempo rimanente
	remaining := time.Until(ctx.Tokens.ExpiresAt).Round(time.Second)
	refreshRemaining := time.Until(ctx.Tokens.RefreshExpiresAt).Round(time.Minute)

	fmt.Printf("\n👤 Utente corrente\n")
	fmt.Printf("   %-16s %s\n", "Username:", ctx.User.Subject)
	fmt.Printf("   %-16s %s\n", "Email:", ctx.User.Email)
	fmt.Printf("   %-16s %s\n", "Nome:", ctx.User.DisplayName)
	fmt.Printf("   %-16s %s\n", "Ruoli:", strings.Join(ctx.User.Roles, ", "))

	fmt.Printf("\n🔗 Connessione\n")
	fmt.Printf("   %-16s %s\n", "Server:", ctx.Server)
	fmt.Printf("   %-16s %s\n", "Realm:", ctx.Realm)
	fmt.Printf("   %-16s %s\n", "Client ID:", ctx.ClientID)

	fmt.Printf("\n🔑 Token\n")
	fmt.Printf("   %-16s %s\n", "Stato:", tokenStatus)
	if remaining > 0 {
		fmt.Printf("   %-16s %s\n", "Scade tra:", remaining)
	}
	fmt.Printf("   %-16s %s\n", "Refresh tra:", refreshRemaining)
	fmt.Printf("   %-16s %s\n", "Contesto:", creds.CurrentContext)
	fmt.Println()

	return nil
}

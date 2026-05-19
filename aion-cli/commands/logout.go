package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/klonet77/aion-cli/internal/config"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Cancella le credenziali locali",
	Long:  `Rimuove i token salvati in ~/.config/aion/credentials.yaml.`,
	RunE:  runLogout,
}

func runLogout(cmd *cobra.Command, args []string) error {
	creds, err := config.LoadCredentials()
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}

	if !creds.IsLoggedIn() {
		fmt.Println("ℹ️  Non sei autenticato.")
		return nil
	}

	// recupera info utente prima di cancellare
	ctx, _ := creds.CurrentContextConfig()
	username := ""
	if ctx != nil {
		username = ctx.User.Subject
	}

	// cancella i token del contesto corrente
	delete(creds.Contexts, creds.CurrentContext)

	if err := config.SaveCredentials(creds); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	if username != "" {
		fmt.Printf("👋 Logout completato per %s.\n", username)
	} else {
		fmt.Println("👋 Logout completato.")
	}

	return nil
}

var logoutAllCmd = &cobra.Command{
	Use:   "logout-all",
	Short: "Cancella tutte le credenziali e il file di configurazione",
	RunE:  runLogoutAll,
}

func runLogoutAll(cmd *cobra.Command, args []string) error {
	path, err := config.CredentialsPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("ℹ️  Nessun file di credenziali trovato.")
			return nil
		}
		return fmt.Errorf("remove credentials: %w", err)
	}

	fmt.Println("🗑️  File credenziali rimosso.")
	return nil
}

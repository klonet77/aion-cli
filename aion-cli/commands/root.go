package commands

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aion",
	Short: "aion-cli — CLI per aion-api",
	Long: `aion è la CLI ufficiale per gestire la piattaforma aion.

Esempi:
  aion login          # autenticati con il browser
  aion whoami         # mostra utente corrente
  aion logout         # cancella le credenziali locali`,
}

// Execute è il punto di ingresso chiamato da main.go.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(logoutAllCmd)
	rootCmd.AddCommand(whoamiCmd)
	rootCmd.AddCommand(pingCmd)
	rootCmd.AddCommand(metricsCmd)
}

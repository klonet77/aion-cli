package commands

import (
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/klonet77/aion-cli/internal/api"
	"github.com/klonet77/aion-cli/internal/config"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Mostra le metriche di aion-api (richiede auth)",
	RunE:  runMetrics,
}

func runMetrics(cmd *cobra.Command, args []string) error {
	creds, err := config.LoadCredentials()
	if err != nil {
		return err
	}

	if !creds.IsLoggedIn() {
		return fmt.Errorf("non autenticato — esegui 'aion login'")
	}

	client := api.NewClient(creds, "https://argo.datainf.cloud")

	baseURL, err := client.BaseURL()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", baseURL+"/apiv1/metrics", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("chiamata fallita: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	fmt.Printf("Status: %d\n\n", resp.StatusCode)
	fmt.Printf("%s\n", string(body))

	return nil
}

package commands

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/klonet77/aion-cli/internal/config"
)

var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Verifica connessione con aion-api",
	RunE:  runPing,
}

func runPing(cmd *cobra.Command, args []string) error {
	creds, err := config.LoadCredentials()
	if err != nil {
		return err
	}

	ctx, err := creds.CurrentContextConfig()
	if err != nil {
		return err
	}

	url := ctx.Server + "/apiv1/monitor/healthz"
	fmt.Printf("🔗 %s\n", url)

	start := time.Now()
	resp, err := http.Get(url)
	elapsed := time.Since(start)

	if err != nil {
		return fmt.Errorf("❌ connessione fallita: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("✅ %d OK  (%s)\n", resp.StatusCode, elapsed.Round(time.Millisecond))
		fmt.Printf("   %s\n", string(body))
	} else {
		fmt.Printf("⚠️  %d  (%s)\n", resp.StatusCode, elapsed.Round(time.Millisecond))
		fmt.Printf("   %s\n", string(body))
	}

	return nil
}

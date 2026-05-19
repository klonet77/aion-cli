package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigDir = ".config/aion"
	CredentialsFile  = "credentials.yaml"
	DefaultContext   = "default"
)

// =============================================================
// Strutture
// =============================================================

type Credentials struct {
	Version        int                       `yaml:"version"`
	CurrentContext string                    `yaml:"current_context"`
	Contexts       map[string]*ContextConfig `yaml:"contexts"`
}

type ContextConfig struct {
	Server   string      `yaml:"server"`
	Realm    string      `yaml:"realm"`
	ClientID string      `yaml:"client_id"`
	Tokens   TokenConfig `yaml:"tokens"`
	User     UserInfo    `yaml:"user"`
}

type TokenConfig struct {
	AccessToken      string    `yaml:"access_token"`
	RefreshToken     string    `yaml:"refresh_token"`
	ExpiresAt        time.Time `yaml:"expires_at"`
	RefreshExpiresAt time.Time `yaml:"refresh_expires_at"`
}

type UserInfo struct {
	Subject     string   `yaml:"subject"`
	Email       string   `yaml:"email"`
	DisplayName string   `yaml:"display_name"`
	Roles       []string `yaml:"roles"`
}

// =============================================================
// Path helpers
// =============================================================

func CredentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home dir: %w", err)
	}
	return filepath.Join(home, DefaultConfigDir, CredentialsFile), nil
}

// =============================================================
// Load
// =============================================================

func LoadCredentials() (*Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Credentials{
			Version:        1,
			CurrentContext: DefaultContext,
			Contexts:       make(map[string]*ContextConfig),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}

	var creds Credentials
	if err := yaml.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	if creds.Contexts == nil {
		creds.Contexts = make(map[string]*ContextConfig)
	}

	return &creds, nil
}

// =============================================================
// Save
// =============================================================

func SaveCredentials(creds *Credentials) error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(creds)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	// permessi restrittivi — solo il proprietario può leggere
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}

	return nil
}

// =============================================================
// Helpers
// =============================================================

func (c *Credentials) CurrentContextConfig() (*ContextConfig, error) {
	ctx, ok := c.Contexts[c.CurrentContext]
	if !ok {
		return nil, fmt.Errorf("context %q not found — run 'aion login' first", c.CurrentContext)
	}
	return ctx, nil
}

func (c *Credentials) IsLoggedIn() bool {
	ctx, ok := c.Contexts[c.CurrentContext]
	if !ok {
		return false
	}
	return ctx.Tokens.AccessToken != "" || ctx.Tokens.RefreshToken != ""
}

func (t *TokenConfig) NeedsRefresh() bool {
	if t.AccessToken == "" {
		return true
	}
	return time.Now().Add(30 * time.Second).After(t.ExpiresAt)
}

func (t *TokenConfig) RefreshExpired() bool {
	if t.RefreshToken == "" {
		return true
	}
	return time.Now().After(t.RefreshExpiresAt)
}

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/orchids"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// OrchidsAuthenticator implements manual token import for Orchids.
type OrchidsAuthenticator struct{}

// NewOrchidsAuthenticator constructs an Orchids authenticator.
func NewOrchidsAuthenticator() *OrchidsAuthenticator {
	return &OrchidsAuthenticator{}
}

// Provider returns the provider key for the authenticator.
func (a *OrchidsAuthenticator) Provider() string {
	return "orchids"
}

// RefreshLead indicates how soon before expiry a refresh should be attempted.
func (a *OrchidsAuthenticator) RefreshLead() *time.Duration {
	d := 1 * time.Minute
	return &d
}

// Login performs manual Orchids token import.
func (a *OrchidsAuthenticator) Login(ctx context.Context, cfg *config.Config, opts *LoginOptions) (*coreauth.Auth, error) {
	_ = ctx
	_ = cfg
	if opts == nil {
		opts = &LoginOptions{}
	}

	token := ""
	if opts.Metadata != nil {
		token = strings.TrimSpace(opts.Metadata["token"])
	}
	if token == "" && opts.Prompt != nil {
		input, err := opts.Prompt("Paste the Orchids __client token: ")
		if err != nil {
			return nil, err
		}
		token = strings.TrimSpace(input)
	}
	if token == "" {
		return nil, fmt.Errorf("orchids auth: token is required")
	}

	cred, err := orchids.ParseCredentials(token)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	hash := sha256.Sum256([]byte(cred.ClientJWT))
	shortID := hex.EncodeToString(hash[:8])
	fileName := fmt.Sprintf("orchids-%s.json", shortID)

	metadata := map[string]any{
		"type":        "orchids",
		"client_jwt":  cred.ClientJWT,
		"imported_at": now.UTC().Format(time.RFC3339),
	}
	if cred.RotatingToken != "" {
		metadata["rotating_token"] = cred.RotatingToken
	}

	return &coreauth.Auth{
		ID:        fileName,
		Provider:  a.Provider(),
		FileName:  fileName,
		Label:     "orchids",
		Status:    coreauth.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  metadata,
	}, nil
}

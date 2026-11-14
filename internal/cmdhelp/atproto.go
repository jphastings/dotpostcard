package cmdhelp

import (
	"fmt"
	"os"

	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/spf13/cobra"
)

// ATCredentials resolves the --at-user/--at-password values, falling back to the same
// environment variables goat checks, in the same order.
func ATCredentials(user, password string) (string, string, error) {
	if user == "" {
		user = firstEnv("GOAT_USERNAME", "ATP_USERNAME", "ATP_AUTH_USERNAME")
	}
	if password == "" {
		password = firstEnv("GOAT_PASSWORD", "ATP_PASSWORD", "ATP_AUTH_PASSWORD")
	}

	if user == "" || password == "" {
		return "", "", fmt.Errorf("uploading to atproto needs credentials: pass --at-user/--at-password, or set GOAT_USERNAME/GOAT_PASSWORD (or ATP_USERNAME/ATP_PASSWORD, or ATP_AUTH_USERNAME/ATP_AUTH_PASSWORD)")
	}
	return user, password, nil
}

// AddATFlags registers --at-user and --at-password on cmd.
func AddATFlags(cmd *cobra.Command) {
	cmd.Flags().String("at-user", "", "atproto handle or DID to act as (falls back to $GOAT_USERNAME/$ATP_USERNAME/$ATP_AUTH_USERNAME)")
	cmd.Flags().String("at-password", "", "atproto PDS app password (falls back to $GOAT_PASSWORD/$ATP_PASSWORD/$ATP_AUTH_PASSWORD)")
}

// ATLogin logs in with the credentials from cmd's AddATFlags flags (or the environment).
func ATLogin(cmd *cobra.Command) (*atproto.Client, error) {
	user, _ := cmd.Flags().GetString("at-user")
	password, _ := cmd.Flags().GetString("at-password")
	user, password, err := ATCredentials(user, password)
	if err != nil {
		return nil, err
	}
	client, err := atproto.Login(user, password, ATPDSHostOverride(), ATPLCHostOverride())
	if err != nil {
		return nil, fmt.Errorf("logging in to atproto: %w", err)
	}
	return client, nil
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}

func ATPDSHostOverride() string { return os.Getenv("ATP_PDS_HOST") }
func ATPLCHostOverride() string { return os.Getenv("ATP_PLC_HOST") }

package cmdhelp

import (
	"fmt"
	"os"
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

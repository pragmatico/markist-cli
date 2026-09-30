package cli

import "os"

// DefaultAPIURL is used when no flag, env var or config value is set.
const DefaultAPIURL = "https://markist.xyz"

// EnvAPIURL is the env var override, ranked between --api-url and the
// config file.
const EnvAPIURL = "MARKIST_API_URL"

// resolveAPIURL implements the --api-url precedence from plan Task 9:
// --api-url flag > MARKIST_API_URL env var > config file value > default.
// configValue is whatever internal/config.Config.APIURL held on disk (empty
// if unset); callers that don't have a config loaded yet can pass "".
func resolveAPIURL(flagValue, configValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv(EnvAPIURL); env != "" {
		return env
	}
	if configValue != "" {
		return configValue
	}
	return DefaultAPIURL
}

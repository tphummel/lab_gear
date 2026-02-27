package config

import "strings"

// ResolveProviderConfig applies provider configuration precedence where explicit
// configuration values override environment values. Empty explicit values are
// treated as unset.
func ResolveProviderConfig(configEndpoint, configAPIKey, envEndpoint, envAPIKey string) (endpoint, apiKey string) {
	endpoint = strings.TrimSpace(envEndpoint)
	apiKey = strings.TrimSpace(envAPIKey)

	if value := strings.TrimSpace(configEndpoint); value != "" {
		endpoint = value
	}
	if value := strings.TrimSpace(configAPIKey); value != "" {
		apiKey = value
	}

	return endpoint, apiKey
}

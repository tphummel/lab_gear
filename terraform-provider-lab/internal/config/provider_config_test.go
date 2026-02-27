package config

import "testing"

func TestResolveProviderConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		configEndpoint string
		configAPIKey   string
		envEndpoint    string
		envAPIKey      string
		wantEndpoint   string
		wantAPIKey     string
	}{
		{
			name:           "uses environment when explicit config is empty",
			configEndpoint: "",
			configAPIKey:   "",
			envEndpoint:    "https://env.example",
			envAPIKey:      "env-token",
			wantEndpoint:   "https://env.example",
			wantAPIKey:     "env-token",
		},
		{
			name:           "explicit config overrides environment",
			configEndpoint: "https://config.example",
			configAPIKey:   "config-token",
			envEndpoint:    "https://env.example",
			envAPIKey:      "env-token",
			wantEndpoint:   "https://config.example",
			wantAPIKey:     "config-token",
		},
		{
			name:           "whitespace explicit values are treated as unset",
			configEndpoint: "   ",
			configAPIKey:   "\t",
			envEndpoint:    "https://env.example",
			envAPIKey:      "env-token",
			wantEndpoint:   "https://env.example",
			wantAPIKey:     "env-token",
		},
		{
			name:           "trims values from both config and env",
			configEndpoint: " https://config.example/ ",
			configAPIKey:   " config-token ",
			envEndpoint:    " https://env.example ",
			envAPIKey:      " env-token ",
			wantEndpoint:   "https://config.example/",
			wantAPIKey:     "config-token",
		},
		{
			name:           "returns empty values when both sources missing",
			configEndpoint: "",
			configAPIKey:   "",
			envEndpoint:    "",
			envAPIKey:      "",
			wantEndpoint:   "",
			wantAPIKey:     "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotEndpoint, gotAPIKey := ResolveProviderConfig(tt.configEndpoint, tt.configAPIKey, tt.envEndpoint, tt.envAPIKey)
			if gotEndpoint != tt.wantEndpoint {
				t.Fatalf("endpoint mismatch: got %q want %q", gotEndpoint, tt.wantEndpoint)
			}
			if gotAPIKey != tt.wantAPIKey {
				t.Fatalf("api key mismatch: got %q want %q", gotAPIKey, tt.wantAPIKey)
			}
		})
	}
}

package main

import (
	"os"
	"testing"
)

// helper that clears the three config env vars and restores them after the test.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	vars := []string{"API_TOKEN", "DB_PATH", "PORT"}
	saved := make(map[string]string, len(vars))
	for _, v := range vars {
		saved[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	t.Cleanup(func() {
		for k, val := range saved {
			if val == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, val)
			}
		}
	})
}

func TestLoadConfig_MissingToken(t *testing.T) {
	clearConfigEnv(t)

	_, _, _, err := loadConfig()
	if err == nil {
		t.Fatal("expected error when API_TOKEN is unset, got nil")
	}
}

func TestLoadConfig_DefaultDBPath(t *testing.T) {
	clearConfigEnv(t)
	os.Setenv("API_TOKEN", "my-token")

	_, dbPath, _, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dbPath != "./lab_gear.db" {
		t.Errorf("DB_PATH default: got %q, want ./lab_gear.db", dbPath)
	}
}

func TestLoadConfig_DefaultPort(t *testing.T) {
	clearConfigEnv(t)
	os.Setenv("API_TOKEN", "my-token")

	_, _, port, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != "8080" {
		t.Errorf("PORT default: got %q, want 8080", port)
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	clearConfigEnv(t)
	os.Setenv("API_TOKEN", "secret")
	os.Setenv("DB_PATH", "/data/lab.db")
	os.Setenv("PORT", "9090")

	token, dbPath, port, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "secret" {
		t.Errorf("token: got %q, want secret", token)
	}
	if dbPath != "/data/lab.db" {
		t.Errorf("dbPath: got %q, want /data/lab.db", dbPath)
	}
	if port != "9090" {
		t.Errorf("port: got %q, want 9090", port)
	}
}

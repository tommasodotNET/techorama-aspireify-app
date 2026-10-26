package main

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

func clearConfig(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"BOOKS_URI", "BOOKS_ACCOUNTKEY", "BOOKS_DATABASENAME", "BOOKS_CONTAINERNAME",
		"COSMOS_ENDPOINT", "COSMOS_KEY", "COSMOS_DATABASE", "COSMOS_CONTAINER", "COSMOS_ALLOW_HTTP",
		"HTTP_ADDR", "PORT",
	} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

func setReference(t *testing.T) {
	t.Helper()
	t.Setenv("BOOKS_URI", "https://localhost:49231")
	t.Setenv("BOOKS_ACCOUNTKEY", base64.StdEncoding.EncodeToString([]byte("test-key")))
	t.Setenv("BOOKS_DATABASENAME", "reference-database")
	t.Setenv("BOOKS_CONTAINERNAME", "reference-container")
}

func TestCosmosReferenceConfiguration(t *testing.T) {
	clearConfig(t)
	setReference(t)
	t.Setenv("COSMOS_ENDPOINT", "https://wrong-account.invalid")
	t.Setenv("COSMOS_KEY", "wrong-key")
	t.Setenv("COSMOS_DATABASE", "wrong-database")
	t.Setenv("COSMOS_CONTAINER", "wrong-container")
	config, err := loadCosmosConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Endpoint != "https://localhost:49231" || config.Database != "reference-database" || config.Container != "reference-container" || config.AllowHTTP {
		t.Fatal("Aspire reference properties were not used")
	}
	store, err := newCosmosStore(config)
	if err != nil {
		t.Fatal(err)
	}
	if store.database.ID() != "reference-database" || store.container.ID() != "reference-container" {
		t.Fatal("Cosmos clients did not use reference database/container names")
	}
}

func TestPartialReferenceDoesNotMixWithStandaloneConfiguration(t *testing.T) {
	for _, missing := range []string{"BOOKS_URI", "BOOKS_ACCOUNTKEY", "BOOKS_DATABASENAME", "BOOKS_CONTAINERNAME"} {
		t.Run(missing, func(t *testing.T) {
			clearConfig(t)
			setReference(t)
			t.Setenv(missing, "")
			t.Setenv("COSMOS_ENDPOINT", "https://wrong-account.invalid")
			t.Setenv("COSMOS_KEY", "standalone-key")
			_, err := loadCosmosConfig()
			if err == nil || !strings.Contains(err.Error(), missing+" is required") {
				t.Fatalf("missing reference configuration should fail explicitly: %v", err)
			}
		})
	}
}

func TestStandaloneCosmosConfiguration(t *testing.T) {
	clearConfig(t)
	t.Setenv("COSMOS_ENDPOINT", "http://cosmos:8081")
	t.Setenv("COSMOS_KEY", "demo-key")
	t.Setenv("COSMOS_ALLOW_HTTP", "true")
	config, err := loadCosmosConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Endpoint != "http://cosmos:8081" || config.Key != "demo-key" || config.Database != "catbooks" || config.Container != "books" || !config.AllowHTTP {
		t.Fatal("standalone configuration changed")
	}
}

func TestInvalidCosmosConfiguration(t *testing.T) {
	for _, endpoint := range []string{"", "not-a-url", "ftp://cosmos:8081", "http://cosmos:8081"} {
		t.Run(endpoint, func(t *testing.T) {
			clearConfig(t)
			t.Setenv("COSMOS_ENDPOINT", endpoint)
			t.Setenv("COSMOS_KEY", "demo-key")
			if _, err := loadCosmosConfig(); err == nil {
				t.Fatal("invalid or insecure endpoint was accepted")
			}
		})
	}
	t.Run("missing key", func(t *testing.T) {
		clearConfig(t)
		t.Setenv("COSMOS_ENDPOINT", "https://cosmos:8081")
		if _, err := loadCosmosConfig(); err == nil || !strings.Contains(err.Error(), "COSMOS_KEY") {
			t.Fatalf("missing key should fail: %v", err)
		}
	})
	t.Run("invalid base64", func(t *testing.T) {
		clearConfig(t)
		setReference(t)
		t.Setenv("BOOKS_ACCOUNTKEY", "not-a-base64-key")
		config, err := loadCosmosConfig()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := newCosmosStore(config); err == nil {
			t.Fatal("invalid account key was accepted")
		}
	})
}

func TestListenAddress(t *testing.T) {
	for _, tc := range []struct {
		name, port, address, want string
		wantError                 bool
	}{
		{"Aspire managed port", "49231", "", ":49231", false},
		{"Compose override", "49231", ":8080", ":8080", false},
		{"standalone default", "", "", ":8080", false},
		{"invalid port", "invalid", "", "", true},
		{"zero port", "0", "", "", true},
		{"negative port", "-1", "", "", true},
		{"out of range", "65536", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearConfig(t)
			t.Setenv("PORT", tc.port)
			t.Setenv("HTTP_ADDR", tc.address)
			got, err := listenAddress()
			if (err != nil) != tc.wantError || got != tc.want {
				t.Fatalf("listen address = %q, error = %v", got, err)
			}
		})
	}
}

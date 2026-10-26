package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

func TestCosmosResetIntegration(t *testing.T) {
	if os.Getenv("COSMOS_INTEGRATION_TEST") != "true" {
		t.Skip("set COSMOS_INTEGRATION_TEST=true with local emulator connection settings")
	}
	config, err := loadCosmosConfig()
	if err != nil {
		t.Fatal(err)
	}
	endpoint, _ := url.Parse(config.Endpoint)
	if host := endpoint.Hostname(); host != "localhost" && host != "127.0.0.1" {
		t.Fatal("this integration test only targets a local emulator")
	}
	config.Database = fmt.Sprintf("reset-test-%d", time.Now().UnixNano())
	store, err := newCosmosStore(config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := store.client.CreateDatabase(ctx, azcosmos.DatabaseProperties{ID: config.Database}, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := store.database.Delete(cleanup, nil); err != nil {
			t.Errorf("delete temporary database %s: %v", config.Database, err)
		}
	})
	if err := store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.database.CreateContainer(ctx, azcosmos.ContainerProperties{
		ID: "untouched", PartitionKeyDefinition: azcosmos.PartitionKeyDefinition{Paths: []string{"/shelf"}},
	}, nil); err != nil {
		t.Fatal(err)
	}
	untouched, err := store.database.NewContainer("untouched")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, sampleBooks()[0].ID); err != nil {
		t.Fatal(err)
	}
	custom := sampleBooks()[0]
	custom.ID = "custom-listing"
	if err := store.Create(ctx, custom); err != nil {
		t.Fatal(err)
	}
	h := handler(store, "test-password")
	for range 2 {
		r := httptest.NewRequest("POST", "/api/admin/database/reset", nil)
		r.SetBasicAuth("admin", "test-password")
		r.Header.Set("X-Confirm-Reset", "clear-and-seed")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("reset: %d %s", w.Code, w.Body.String())
		}
		if err := store.Initialize(ctx); err != nil {
			t.Fatal(err)
		}
		books, err := store.List(ctx)
		if err != nil || len(books) != len(sampleBooks()) {
			t.Fatalf("reset/seed did not restore exactly four samples: %v, %d books", err, len(books))
		}
		for _, book := range books {
			found := false
			for _, sample := range sampleBooks() {
				if book == sample {
					found = true
				}
			}
			if !found {
				t.Fatalf("unexpected book after reset: %+v", book)
			}
		}
		if _, err := untouched.Read(ctx, nil); err != nil {
			t.Fatalf("reset affected another container: %v", err)
		}
	}
}

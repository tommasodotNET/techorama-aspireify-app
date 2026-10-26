package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

var shelf = azcosmos.NewPartitionKeyString("books")

type cosmosStore struct {
	client    *azcosmos.Client
	database  *azcosmos.DatabaseClient
	container *azcosmos.ContainerClient
}

func newCosmosStore() (*cosmosStore, error) {
	credential, err := azcosmos.NewKeyCredential(os.Getenv("COSMOS_KEY"))
	if err != nil {
		return nil, err
	}
	client, err := azcosmos.NewClientWithKey(os.Getenv("COSMOS_ENDPOINT"), credential, &azcosmos.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			InsecureAllowCredentialWithHTTP: os.Getenv("COSMOS_ALLOW_HTTP") == "true",
			Retry:                           policy.RetryOptions{MaxRetries: 2, TryTimeout: 5 * time.Second},
			Telemetry:                       policy.TelemetryOptions{Disabled: true},
		},
	})
	if err != nil {
		return nil, err
	}
	database, err := client.NewDatabase(env("COSMOS_DATABASE", "catbooks"))
	if err != nil {
		return nil, err
	}
	container, err := database.NewContainer(env("COSMOS_CONTAINER", "books"))
	if err != nil {
		return nil, err
	}
	return &cosmosStore{client, database, container}, nil
}

func hasStatus(err error, status int) bool {
	var response *azcore.ResponseError
	return errors.As(err, &response) && response.StatusCode == status
}

func (s *cosmosStore) Initialize(ctx context.Context) error {
	_, err := s.client.CreateDatabase(ctx, azcosmos.DatabaseProperties{ID: s.database.ID()}, nil)
	if err != nil && !hasStatus(err, http.StatusConflict) {
		return err
	}
	_, err = s.database.CreateContainer(ctx, azcosmos.ContainerProperties{
		ID: s.container.ID(),
		PartitionKeyDefinition: azcosmos.PartitionKeyDefinition{
			Paths: []string{"/shelf"}, Kind: azcosmos.PartitionKeyKindHash, Version: 2,
		},
	}, nil)
	if err != nil && !hasStatus(err, http.StatusConflict) {
		return err
	}
	return s.seed(ctx)
}

func (s *cosmosStore) seed(ctx context.Context) error {
	_, err := s.container.ReadItem(ctx, shelf, "_seed-v1", nil)
	if err == nil {
		return nil
	}
	if !hasStatus(err, http.StatusNotFound) {
		return err
	}
	// Commit the marker and samples together: restarts never resurrect removed books.
	batch := s.container.NewTransactionalBatch(shelf)
	for _, book := range sampleBooks() {
		data, err := json.Marshal(book)
		if err != nil {
			return err
		}
		batch.CreateItem(data, nil)
	}
	batch.CreateItem([]byte(`{"id":"_seed-v1","shelf":"books","kind":"seed"}`), nil)
	response, err := s.container.ExecuteTransactionalBatch(ctx, batch, nil)
	if err != nil {
		return err
	}
	if !response.Success {
		// Another API instance may have completed the same initial transaction.
		if _, err := s.container.ReadItem(ctx, shelf, "_seed-v1", nil); err != nil {
			return fmt.Errorf("sample seed transaction failed: %+v: %w", response.OperationResults, err)
		}
	}
	return nil
}

func sampleBooks() []Book {
	return []Book{
		{"sample-sunbeam", "books", "book", "The Art of the Sunbeam", "Miso Whiskers", "Miso", 1299, "A practical guide to finding the warmest patch of floor. Includes advanced napping techniques."},
		{"sample-tuna", "books", "book", "Twenty Thousand Leagues Under the Tuna", "Captain Claw", "Captain Claw", 1599, "An ocean-sized adventure for the cat who dreams beyond the fishbowl."},
		{"sample-keyboard", "books", "book", "A Room of One's Meow", "Virginia Woof (a cat, actually)", "Cleo", 999, "On independence, cardboard boxes, and the importance of sitting on your human's keyboard."},
		{"sample-midnight", "books", "book", "Midnight Zoomies", "Edgar Allan Paw", "Luna", 1850, "Four paws. One hallway. No brakes. A collection of extremely short thrillers."},
	}
}

func (s *cosmosStore) List(ctx context.Context) ([]Book, error) {
	books := []Book{}
	pager := s.container.NewQueryItemsPager("SELECT * FROM c WHERE c.kind = 'book'", shelf, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Items {
			var book Book
			if err := json.Unmarshal(item, &book); err != nil {
				return nil, fmt.Errorf("decode stored book: %w", err)
			}
			books = append(books, book)
		}
	}
	sort.Slice(books, func(i, j int) bool {
		if books[i].Title == books[j].Title {
			return books[i].ID < books[j].ID
		}
		return books[i].Title < books[j].Title
	})
	return books, nil
}

func (s *cosmosStore) Create(ctx context.Context, book Book) error {
	data, err := json.Marshal(book)
	if err != nil {
		return err
	}
	_, err = s.container.CreateItem(ctx, shelf, data, nil)
	return err
}

func (s *cosmosStore) Delete(ctx context.Context, id string) error {
	_, err := s.container.DeleteItem(ctx, shelf, id, nil)
	if hasStatus(err, http.StatusNotFound) {
		return errNotFound
	}
	return err
}

func (s *cosmosStore) Ready(ctx context.Context) error {
	_, err := s.container.Read(ctx, nil)
	return err
}

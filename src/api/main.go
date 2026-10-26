package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Book struct {
	ID          string `json:"id"`
	Shelf       string `json:"shelf"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	Seller      string `json:"seller"`
	PriceCents  int    `json:"priceCents"`
	Description string `json:"description"`
}

type BookInput struct {
	Title       string `json:"title"`
	Author      string `json:"author"`
	Seller      string `json:"seller"`
	PriceCents  int    `json:"priceCents"`
	Description string `json:"description"`
}

var errNotFound = errors.New("book not found")

type bookStore interface {
	List(context.Context) ([]Book, error)
	Create(context.Context, Book) error
	Delete(context.Context, string) error
	Ready(context.Context) error
	Reset(context.Context) error
}

func handler(store bookStore, password string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := store.Ready(ctx); err != nil {
			log.Printf("readiness: %v", err)
			writeError(w, http.StatusServiceUnavailable, "The bookshelf is temporarily offline.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /api/books", func(w http.ResponseWriter, r *http.Request) {
		books, err := store.List(r.Context())
		if err != nil {
			log.Printf("list books: %v", err)
			writeError(w, http.StatusServiceUnavailable, "Could not load the bookshelf. Please retry.")
			return
		}
		if books == nil {
			books = []Book{}
		}
		writeJSON(w, http.StatusOK, books)
	})
	admin := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user, supplied, ok := r.BasicAuth()
			expectedHash, suppliedHash := sha256.Sum256([]byte(password)), sha256.Sum256([]byte(supplied))
			if !ok || user != "admin" || subtle.ConstantTimeCompare(expectedHash[:], suppliedHash[:]) != 1 {
				writeError(w, http.StatusUnauthorized, "Incorrect admin password.")
				return
			}
			next(w, r)
		}
	}
	mux.HandleFunc("POST /api/admin/check", admin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("POST /api/admin/database/reset", admin(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Confirm-Reset") != "clear-and-seed" {
			writeError(w, http.StatusBadRequest, "Confirm deletion with X-Confirm-Reset: clear-and-seed.")
			return
		}
		if err := store.Reset(r.Context()); err != nil {
			log.Printf("reset books container: %v", err)
			writeError(w, http.StatusServiceUnavailable, "Could not reset the bookshelf. The reset may be incomplete; retry the command.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"message":     "Books container cleared and sample books restored.",
			"sampleBooks": len(sampleBooks()),
		})
	}))
	mux.HandleFunc("POST /api/books", admin(func(w http.ResponseWriter, r *http.Request) {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "Send application/json.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var input BookInput
		if err := decoder.Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "Send one valid book as JSON (maximum 16 KB).")
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			writeError(w, http.StatusBadRequest, "Send exactly one JSON object.")
			return
		}
		if err := input.validate(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			log.Printf("generate book ID: %v", err)
			writeError(w, http.StatusInternalServerError, "Could not create a book ID.")
			return
		}
		book := Book{hex.EncodeToString(id[:]), "books", "book", input.Title, input.Author, input.Seller, input.PriceCents, input.Description}
		if err := store.Create(r.Context(), book); err != nil {
			log.Printf("create book: %v", err)
			writeError(w, http.StatusServiceUnavailable, "Could not save the book. Check the shelf before retrying.")
			return
		}
		writeJSON(w, http.StatusCreated, book)
	}))
	mux.HandleFunc("DELETE /api/books/{id}", admin(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "_seed-v1" || strings.ContainsAny(id, "/\\?#") || len(id) > 100 {
			writeError(w, http.StatusBadRequest, "Invalid book ID.")
			return
		}
		err := store.Delete(r.Context(), id)
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "That book is no longer on the shelf.")
		} else if err != nil {
			log.Printf("delete book: %v", err)
			writeError(w, http.StatusServiceUnavailable, "Could not remove the book. Please retry.")
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	traced := otelhttp.NewHandler(mux, "api",
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/healthz" }),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			if r.Pattern != "" {
				return r.Pattern
			}
			return r.Method
		}),
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		traced.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (b *BookInput) validate() error {
	b.Title, b.Author, b.Seller, b.Description = strings.TrimSpace(b.Title), strings.TrimSpace(b.Author), strings.TrimSpace(b.Seller), strings.TrimSpace(b.Description)
	for _, field := range []struct {
		name, value string
		max         int
	}{{"Title", b.Title, 120}, {"Author", b.Author, 100}, {"Cat seller", b.Seller, 80}} {
		if utf8.RuneCountInString(field.value) == 0 || utf8.RuneCountInString(field.value) > field.max {
			return fmt.Errorf("%s must be 1-%d characters.", field.name, field.max)
		}
	}
	if b.PriceCents < 1 || b.PriceCents > 100000 {
		return errors.New("Price must be between $0.01 and $1,000.00.")
	}
	if utf8.RuneCountInString(b.Description) > 500 {
		return errors.New("Description must be at most 500 characters.")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func run() error {
	config, err := loadCosmosConfig()
	if err != nil {
		return err
	}
	address, err := listenAddress()
	if err != nil {
		return err
	}
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		password = demoAdminPassword
		log.Print("WARNING: ADMIN_PASSWORD is unset; using the public local-demo password. Do not expose this app to untrusted networks.")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	shutdownTracing, err := initTracing(ctx)
	if err != nil {
		return err
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdownTracing(shutdown); err != nil {
			log.Printf("tracing shutdown: %v", err)
		}
	}()
	store, err := newCosmosStore(config)
	if err != nil {
		return err
	}
	startup, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	for {
		err = store.Initialize(startup)
		if err == nil {
			break
		}
		log.Printf("waiting for Cosmos DB: %v", err)
		select {
		case <-startup.Done():
			return fmt.Errorf("database initialization: %w", startup.Err())
		case <-time.After(2 * time.Second):
		}
	}
	server := &http.Server{
		Addr: address, Handler: handler(store, password),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if err := server.Shutdown(shutdown); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()
	log.Printf("Purr & Prose API listening on %s", server.Addr)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		<-shutdownDone
		return nil
	}
	return err
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

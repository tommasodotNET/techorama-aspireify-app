package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type memoryStore struct {
	books map[string]Book
	fail  bool
}

func (s *memoryStore) List(context.Context) ([]Book, error) {
	if s.fail {
		return nil, errors.New("database unavailable")
	}
	books := []Book{}
	for _, book := range s.books {
		books = append(books, book)
	}
	return books, nil
}
func (s *memoryStore) Create(_ context.Context, book Book) error {
	if s.fail {
		return errors.New("database unavailable")
	}
	s.books[book.ID] = book
	return nil
}
func (s *memoryStore) Delete(_ context.Context, id string) error {
	if s.fail {
		return errors.New("database unavailable")
	}
	if _, exists := s.books[id]; !exists {
		return errNotFound
	}
	delete(s.books, id)
	return nil
}
func (s *memoryStore) Ready(context.Context) error {
	if s.fail {
		return errors.New("database unavailable")
	}
	return nil
}
func (s *memoryStore) Reset(context.Context) error {
	if s.fail {
		return errors.New("database unavailable")
	}
	s.books = make(map[string]Book)
	for _, book := range sampleBooks() {
		s.books[book.ID] = book
	}
	return nil
}

func request(h http.Handler, method, path, body, password string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if password != "" {
		r.SetBasicAuth("admin", password)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

const validBook = `{"title":"  The Cat in the Stack  ","author":"Miso","seller":"Cleo","priceCents":1250,"description":"A good read."}`

func TestAdminWorkflow(t *testing.T) {
	store := &memoryStore{books: map[string]Book{}}
	h := handler(store, "test-password")
	if w := request(h, "GET", "/api/books", "", ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty shelf: %d %s", w.Code, w.Body.String())
	}
	for _, password := range []string{"", "wrong"} {
		for _, route := range []struct{ method, path string }{{"POST", "/api/admin/check"}, {"POST", "/api/admin/database/reset"}, {"POST", "/api/books"}, {"DELETE", "/api/books/anything"}} {
			if w := request(h, route.method, route.path, validBook, password); w.Code != 401 {
				t.Fatalf("unprotected write: %s %s: %d", route.method, route.path, w.Code)
			}
		}
	}
	if w := request(h, "POST", "/api/admin/check", "", "test-password"); w.Code != 204 {
		t.Fatalf("login: %d", w.Code)
	}
	w := request(h, "POST", "/api/books", validBook, "test-password")
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var book Book
	if err := json.Unmarshal(w.Body.Bytes(), &book); err != nil {
		t.Fatal(err)
	}
	if book.ID == "" || book.Title != "The Cat in the Stack" || book.PriceCents != 1250 {
		t.Fatalf("unexpected saved book: %+v", book)
	}
	list := request(h, "GET", "/api/books", "", "")
	if !bytes.Contains(list.Body.Bytes(), []byte(book.ID)) {
		t.Fatal("saved book not publicly visible")
	}
	if w := request(h, "DELETE", "/api/books/"+book.ID, "", "test-password"); w.Code != 204 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if w := request(h, "DELETE", "/api/books/"+book.ID, "", "test-password"); w.Code != 404 {
		t.Fatalf("missing delete: %d", w.Code)
	}
	if len(store.books) != 0 {
		t.Fatal("book still stored")
	}
}

func TestDatabaseResetRequiresAuthenticationAndConfirmation(t *testing.T) {
	store := &memoryStore{books: map[string]Book{"custom": {ID: "custom", Title: "Keep until confirmed"}}}
	h := handler(store, "test-password")
	if w := request(h, "POST", "/api/admin/database/reset", "", "test-password"); w.Code != http.StatusBadRequest || len(store.books) != 1 {
		t.Fatalf("unconfirmed reset changed data: %d", w.Code)
	}
	for _, password := range []string{"", "wrong", "test-password"} {
		r := httptest.NewRequest("POST", "/api/admin/database/reset", nil)
		r.SetBasicAuth("admin", password)
		r.Header.Set("X-Confirm-Reset", "clear-and-seed")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if password != "test-password" {
			if w.Code != http.StatusUnauthorized || len(store.books) != 1 {
				t.Fatalf("unauthorized reset changed data: %d", w.Code)
			}
			continue
		}
		if w.Code != http.StatusOK || len(store.books) != len(sampleBooks()) || store.books["custom"].ID != "" {
			t.Fatalf("confirmed reset did not restore only samples: %d %s", w.Code, w.Body.String())
		}
		store.fail = true
		failed := httptest.NewRecorder()
		h.ServeHTTP(failed, r.Clone(context.Background()))
		if failed.Code != http.StatusServiceUnavailable || !strings.Contains(failed.Body.String(), "incomplete") {
			t.Fatalf("reset failure was hidden: %d %s", failed.Code, failed.Body.String())
		}
	}
}

func TestInvalidBooks(t *testing.T) {
	for name, body := range map[string]string{
		"empty": `{}`, "malformed": `{`, "null": `null`,
		"trailing": validBook + `{}`, "unknown": strings.Replace(validBook, `"title"`, `"id"`, 1),
		"free":             strings.Replace(validBook, "1250", "0", 1),
		"negative":         strings.Replace(validBook, "1250", "-1", 1),
		"fraction":         strings.Replace(validBook, "1250", "1.5", 1),
		"too expensive":    strings.Replace(validBook, "1250", "100001", 1),
		"long title":       strings.Replace(validBook, "The Cat in the Stack", strings.Repeat("a", 121), 1),
		"blank title":      strings.Replace(validBook, "The Cat in the Stack", " ", 1),
		"long description": strings.Replace(validBook, "A good read.", strings.Repeat("a", 501), 1),
		"oversized":        strings.Replace(validBook, "A good read.", strings.Repeat("a", 17000), 1),
	} {
		t.Run(name, func(t *testing.T) {
			store := &memoryStore{books: map[string]Book{}}
			w := request(handler(store, "test-password"), "POST", "/api/books", body, "test-password")
			if w.Code != 400 || len(store.books) != 0 {
				t.Fatalf("invalid input saved: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestDatabaseFailuresAreNotSuccess(t *testing.T) {
	h := handler(&memoryStore{fail: true}, "test-password")
	for _, route := range []struct{ method, path, body string }{
		{"GET", "/healthz", ""}, {"GET", "/api/books", ""},
		{"POST", "/api/books", validBook}, {"DELETE", "/api/books/book", ""},
	} {
		w := request(h, route.method, route.path, route.body, "test-password")
		if w.Code != 503 || !bytes.Contains(w.Body.Bytes(), []byte(`"error"`)) {
			t.Fatalf("failure not surfaced for %s: %d %s", route.path, w.Code, w.Body.String())
		}
	}
}

func TestSeedMarkerCannotBeDeleted(t *testing.T) {
	w := request(handler(&memoryStore{}, "test-password"), "DELETE", "/api/books/_seed-v1", "", "test-password")
	if w.Code != 400 {
		t.Fatalf("seed marker delete: %d", w.Code)
	}
}

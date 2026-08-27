package web

import (
	"map-registration-gate/internal/app"
	"map-registration-gate/internal/store"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	st, e := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if e != nil {
		t.Fatal(e)
	}
	return New(app.New(st)).Handler()
}
func TestWorkbenchAndBodyLimit(t *testing.T) {
	h := testHandler(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "<canvas") || !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src") {
		t.Fatalf("invalid workbench response: %d", w.Code)
	}
	large := strings.NewReader(strings.Repeat("x", (1<<20)+1))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/jobs", large))
	if w.Code != 400 {
		t.Fatalf("expected bad request, got %d", w.Code)
	}
}
func TestNotFoundMapping(t *testing.T) {
	h := testHandler(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/jobs/missing", nil))
	if w.Code != 404 || !strings.Contains(w.Body.String(), "not_found") {
		t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
	}
}

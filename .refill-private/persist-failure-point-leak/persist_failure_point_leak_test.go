package persistfailurepointleak

import (
	"bytes"
	"encoding/json"
	"map-registration-gate/internal/app"
	"map-registration-gate/internal/store"
	"map-registration-gate/internal/web"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPersistFailureDoesNotLeakPoint(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	st, err := store.Open(statePath)
	if err != nil {
		t.Fatal(err)
	}
	handler := web.New(app.New(st)).Handler()

	create := app.CreateJobCommand{
		RequestID: "persist-create", JobID: "persist-job", Title: "历史图",
		MapYear: 1936, ImageWidth: 100, ImageHeight: 100,
		ImageSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		TargetCRS:   "EPSG:3857", RMSELimit: 1, OperatorID: "operator-a",
	}
	requestJSON(t, handler, http.MethodPost, "/api/jobs", create, http.StatusCreated)
	requestJSON(t, handler, http.MethodPost, "/api/jobs/persist-job/freeze", map[string]any{
		"request_id": "persist-freeze", "expected_revision": 1,
	}, http.StatusOK)

	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(statePath, 0o755); err != nil {
		t.Fatal(err)
	}
	requestJSON(t, handler, http.MethodPost, "/api/jobs/persist-job/points", map[string]any{
		"request_id": "persist-point", "expected_revision": 2, "point_id": "p1",
		"pixel_x": 10, "pixel_y": 10, "map_x": 110, "map_y": 210,
		"evidence_note": "稳定地物", "actor_id": "operator-a",
	}, http.StatusUnprocessableEntity)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/jobs/persist-job", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("query failed: %d %s", recorder.Code, recorder.Body.String())
	}
	var view struct {
		Points []json.RawMessage `json:"points"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Points) != 0 {
		t.Fatalf("TestPersistFailureDoesNotLeakPoint: failed transaction leaked %d point(s)", len(view.Points))
	}
}

func requestJSON(t *testing.T, handler http.Handler, method, path string, body any, want int) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, bytes.NewReader(payload)))
	if recorder.Code != want {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, recorder.Code, want, recorder.Body.String())
	}
}

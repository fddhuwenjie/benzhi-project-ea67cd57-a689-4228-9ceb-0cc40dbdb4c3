package replacement_preview_cache_stale_test

import (
	"errors"
	"path/filepath"
	"testing"

	"map-registration-gate/internal/app"
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/store"
)

func TestReplacementPreviewCacheSeparatesCandidates(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := app.New(s)
	sha := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	result, err := service.CreateJob(app.CreateJobCommand{
		RequestID: "cache-create", JobID: "cache-job", Title: "历史图",
		MapYear: 1936, ImageWidth: 1000, ImageHeight: 800, ImageSHA256: sha,
		TargetCRS: "EPSG:3857", RMSELimit: 3.5, OperatorID: "operator-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err = service.Freeze(app.RevisionCommand{RequestID: "cache-freeze", JobID: result.JobID, ExpectedRevision: result.Revision})
	if err != nil {
		t.Fatal(err)
	}

	points := []struct {
		id             string
		px, py, mx, my float64
	}{
		{"p1", 100, 100, 210, 320},
		{"p2", 500, 100, 1010, 320},
		{"p3", 900, 100, 1810, 320},
		{"p4", 900, 700, 1820, 2140},
		{"p5", 500, 700, 1010, 2120},
		{"p6", 100, 700, 210, 2120},
		{"p7", 100, 400, 210, 1220},
		{"p8", 500, 400, 1010, 1220},
		{"p9", 900, 400, 1810, 1220},
	}
	for _, point := range points {
		result, err = service.AddPoint(app.PointCommand{
			RequestID: "cache-add-" + point.id, JobID: result.JobID, PointID: point.id,
			ExpectedRevision: result.Revision, PixelX: point.px, PixelY: point.py,
			MapX: point.mx, MapY: point.my, EvidenceNote: "控制点证据", ActorID: "operator-a",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	result, err = service.Evaluate(app.RevisionCommand{RequestID: "cache-evaluate", JobID: result.JobID, ExpectedRevision: result.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != string(domain.NeedsFix) {
		t.Fatalf("expected remediation state, got %s", result.State)
	}

	first := app.ReplacePointCommand{
		JobID: result.JobID, OldPointID: "p4", NewPointID: "p4-first",
		ExpectedRevision: result.Revision, PixelX: 900, PixelY: 700, MapX: 1810, MapY: 2120,
		Reason: "首次候选", EvidenceNote: "首次替换证据", ActorID: "operator-a",
	}
	firstPreview, err := service.PreviewReplacement(first)
	if err != nil || !firstPreview.WouldPass {
		t.Fatalf("first preview failed: %+v %v", firstPreview, err)
	}
	second := first
	second.RequestID = "cache-replace-second"
	second.NewPointID = "p4-second"
	second.Reason = "第二次候选"
	second.EvidenceNote = "第二次替换证据"
	secondPreview, err := service.PreviewReplacement(second)
	if err != nil || !secondPreview.WouldPass {
		t.Fatalf("second preview failed: %+v %v", secondPreview, err)
	}
	second.PreviewDigest = secondPreview.PreviewDigest
	_, err = service.ReplacePoint(second)
	if errors.Is(err, domain.ErrConflict) {
		t.Fatal("TestReplacementPreviewCacheSeparatesCandidates: second candidate reused first preview and was rejected as expired")
	}
	if err != nil {
		t.Fatalf("unexpected replacement error: %v", err)
	}
}

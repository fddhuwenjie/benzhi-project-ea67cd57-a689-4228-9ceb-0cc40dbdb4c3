package app

import (
	"errors"
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/store"
	"path/filepath"
	"testing"
)

func testService(t *testing.T) *Service {
	t.Helper()
	s, e := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if e != nil {
		t.Fatal(e)
	}
	return New(s)
}
func createFrozen(t *testing.T, s *Service) (string, uint64) {
	t.Helper()
	sha := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	r, e := s.CreateJob(CreateJobCommand{RequestID: "req-create", JobID: "job", Title: "历史图", MapYear: 1936, ImageWidth: 100, ImageHeight: 100, ImageSHA256: sha, TargetCRS: "EPSG:3857", RMSELimit: .1, OperatorID: "op"})
	if e != nil {
		t.Fatal(e)
	}
	r, e = s.Freeze(RevisionCommand{"req-freeze", r.JobID, r.Revision})
	if e != nil {
		t.Fatal(e)
	}
	return r.JobID, r.Revision
}
func TestIdempotencyAndRevision(t *testing.T) {
	s := testService(t)
	id, rev := createFrozen(t, s)
	cmd := PointCommand{"req-point", id, "p1", rev, 10, 10, 20, 20, "证据", "op"}
	first, e := s.AddPoint(cmd)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.AddPoint(cmd)
	if e != nil || !again.Replayed || again.Revision != first.Revision {
		t.Fatalf("replay failed: %+v %v", again, e)
	}
	cmd.MapX = 99
	if _, e = s.AddPoint(cmd); !errors.Is(e, domain.ErrIdempotency) {
		t.Fatalf("expected idempotency conflict: %v", e)
	}
	cmd.RequestID = "new-request"
	if _, e = s.AddPoint(cmd); !errors.Is(e, domain.ErrConflict) {
		t.Fatalf("expected revision conflict: %v", e)
	}
}
func TestPointChangeInvalidatesEvaluation(t *testing.T) {
	s := testService(t)
	id, rev := createFrozen(t, s)
	pts := []PointCommand{{PointID: "a", PixelX: 10, PixelY: 10, MapX: 20, MapY: 30}, {PointID: "b", PixelX: 90, PixelY: 10, MapX: 180, MapY: 30}, {PointID: "c", PixelX: 10, PixelY: 90, MapX: 20, MapY: 270}, {PointID: "d", PixelX: 90, PixelY: 90, MapX: 180, MapY: 270}}
	for i := range pts {
		pts[i].RequestID = "point-" + pts[i].PointID
		pts[i].JobID = id
		pts[i].ExpectedRevision = rev
		pts[i].EvidenceNote = "证据"
		pts[i].ActorID = "op"
		r, e := s.AddPoint(pts[i])
		if e != nil {
			t.Fatal(e)
		}
		rev = r.Revision
	}
	r, e := s.Evaluate(RevisionCommand{"req-eval", id, rev})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.AddPoint(PointCommand{"req-extra", id, "e", r.Revision, 50, 50, 100, 150, "证据", "op"}); e != nil {
		t.Fatal(e)
	}
	v, e := s.Get(id)
	if e != nil {
		t.Fatal(e)
	}
	if v.Evaluation != nil {
		t.Fatal("stale evaluation retained")
	}
	if _, e = s.SubmitReview(RevisionCommand{"req-submit", id, v.Job.Revision}); !errors.Is(e, domain.ErrRule) {
		t.Fatalf("stale point set submitted: %v", e)
	}
}

func TestDraftBaselineRevisionAndFreeze(t *testing.T) {
	s := testService(t)
	sha := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	r, err := s.CreateJob(CreateJobCommand{RequestID: "draft-create", JobID: "draft", Title: "旧标题", MapYear: 1936, ImageWidth: 100, ImageHeight: 100, ImageSHA256: sha, TargetCRS: "EPSG:3857", RMSELimit: 1, OperatorID: "op"})
	if err != nil {
		t.Fatal(err)
	}
	cmd := ReviseBaselineCommand{RequestID: "draft-revise", JobID: r.JobID, ExpectedRevision: r.Revision, Title: "新标题", MapYear: 1937, ImageWidth: 120, ImageHeight: 90, ImageSHA256: sha, TargetCRS: "EPSG:4490", RMSELimit: .5}
	r, err = s.ReviseBaseline(cmd)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.ReviseBaseline(cmd)
	if err != nil || !again.Replayed {
		t.Fatalf("revision replay failed: %+v %v", again, err)
	}
	view, _ := s.Get(r.JobID)
	if view.Job.Title != "新标题" || len(view.Timeline) != 2 {
		t.Fatalf("baseline revision not persisted: %+v", view.Job)
	}
	r, err = s.Freeze(RevisionCommand{"draft-freeze", r.JobID, r.Revision})
	if err != nil {
		t.Fatal(err)
	}
	cmd.RequestID = "after-freeze"
	cmd.ExpectedRevision = r.Revision
	if _, err = s.ReviseBaseline(cmd); !errors.Is(err, domain.ErrImmutable) {
		t.Fatalf("frozen baseline revised: %v", err)
	}
}

func TestBatchPreflightAndAtomicImport(t *testing.T) {
	s := testService(t)
	id, rev := createFrozen(t, s)
	points := []BatchPoint{{"a", 10, 10, 20, 30, "证据", "op"}, {"b", 90, 10, 180, 30, "证据", "op"}, {"c", 10, 90, 20, 270, "证据", "op"}, {"d", 90, 90, 180, 270, "证据", "op"}}
	cmd := BatchPointsCommand{JobID: id, ExpectedRevision: rev, Points: points}
	preview, err := s.PreviewBatch(cmd)
	if err != nil || len(preview.Errors) != 0 || !preview.Solvable {
		t.Fatalf("unexpected preflight: %+v %v", preview, err)
	}
	cmd.RequestID = "batch-import"
	cmd.PreviewDigest = preview.PreviewDigest
	r, err := s.ImportBatch(cmd)
	if err != nil {
		t.Fatal(err)
	}
	view, _ := s.Get(id)
	if len(view.Points) != 4 || r.Revision != rev+1 || view.Job.State != domain.Solvable {
		t.Fatalf("batch not atomic: %+v", view)
	}
	bad := BatchPointsCommand{JobID: id, ExpectedRevision: r.Revision, Points: []BatchPoint{{"x", 101, 10, 0, 0, "证据", "op"}, {"y", 50, 50, 0, 0, "证据", "op"}, {"z", 50, 50, 1, 1, "证据", "op"}}}
	badPreview, err := s.PreviewBatch(bad)
	if err != nil || len(badPreview.Errors) < 3 {
		t.Fatalf("row errors missing: %+v %v", badPreview, err)
	}
	bad.RequestID = "bad-batch"
	bad.PreviewDigest = badPreview.PreviewDigest
	if _, err = s.ImportBatch(bad); !errors.Is(err, domain.ErrRule) {
		t.Fatalf("invalid batch imported: %v", err)
	}
	after, _ := s.Get(id)
	if len(after.Points) != 4 || len(after.Timeline) != len(view.Timeline) {
		t.Fatal("invalid batch changed aggregate")
	}
}

func TestEvaluationHistoryIsAppendOnly(t *testing.T) {
	s := testService(t)
	id, rev := createFrozen(t, s)
	pts := []PointCommand{{PointID: "a", PixelX: 10, PixelY: 10, MapX: 20, MapY: 30}, {PointID: "b", PixelX: 90, PixelY: 10, MapX: 180, MapY: 30}, {PointID: "c", PixelX: 10, PixelY: 90, MapX: 20, MapY: 270}, {PointID: "d", PixelX: 90, PixelY: 90, MapX: 180, MapY: 270}}
	for i := range pts {
		pts[i].RequestID = "history-" + pts[i].PointID
		pts[i].JobID = id
		pts[i].ExpectedRevision = rev
		pts[i].EvidenceNote = "证据"
		pts[i].ActorID = "op"
		r, err := s.AddPoint(pts[i])
		if err != nil {
			t.Fatal(err)
		}
		rev = r.Revision
	}
	r, err := s.Evaluate(RevisionCommand{"history-eval-1", id, rev})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Evaluate(RevisionCommand{"history-eval-2", id, r.Revision}); err != nil {
		t.Fatal(err)
	}
	view, _ := s.Get(id)
	if len(view.EvaluationHistory) != 2 || view.EvaluationHistory[0].Current || !view.EvaluationHistory[0].Invalidated || !view.EvaluationHistory[1].Current || view.EvaluationHistory[1].Comparison.Trend != "unchanged" {
		t.Fatalf("history projection invalid: %+v", view.EvaluationHistory)
	}
}

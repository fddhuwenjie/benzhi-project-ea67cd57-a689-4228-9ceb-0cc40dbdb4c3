package remediationscope_test

import (
	"testing"

	"map-registration-gate/internal/app"
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/store"
)

func TestReplacementKeepsOtherReturnItemsOpen(t *testing.T) {
	service := newService(t)
	result, err := service.CreateJob(app.CreateJobCommand{
		RequestID:   "create-review-scope",
		JobID:       "review-scope-job",
		Title:       "复核范围测试图",
		MapYear:     1936,
		ImageWidth:  100,
		ImageHeight: 100,
		ImageSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		TargetCRS:   "EPSG:3857",
		RMSELimit:   0.5,
		OperatorID:  "operator-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err = service.Freeze(app.RevisionCommand{"freeze-review-scope", result.JobID, result.Revision})
	mustSucceed(t, err)

	points := []app.PointCommand{
		{PointID: "p1", PixelX: 10, PixelY: 10, MapX: 20, MapY: 30},
		{PointID: "p2", PixelX: 90, PixelY: 10, MapX: 180, MapY: 30},
		{PointID: "p3", PixelX: 10, PixelY: 90, MapX: 20, MapY: 270},
		{PointID: "p4", PixelX: 90, PixelY: 90, MapX: 180, MapY: 270},
	}
	for i := range points {
		points[i].RequestID = "point-review-scope-" + points[i].PointID
		points[i].JobID = result.JobID
		points[i].ExpectedRevision = result.Revision
		points[i].EvidenceNote = "稳定地物证据"
		points[i].ActorID = "operator-a"
		result, err = service.AddPoint(points[i])
		mustSucceed(t, err)
	}
	result, err = service.Evaluate(app.RevisionCommand{"evaluate-review-scope", result.JobID, result.Revision})
	mustSucceed(t, err)
	result, err = service.SubmitReview(app.RevisionCommand{"submit-review-scope", result.JobID, result.Revision})
	mustSucceed(t, err)

	view, err := service.Get(result.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Samples) < 2 {
		t.Fatalf("need at least two review samples, got %v", view.Samples)
	}
	failed := map[string]bool{view.Samples[0]: true, view.Samples[1]: true}
	sampleResults := make(map[string]bool, len(view.Samples))
	items := make([]domain.ReviewItem, 0, len(view.Samples))
	for _, pointID := range view.Samples {
		passed := !failed[pointID]
		sampleResults[pointID] = passed
		item := domain.ReviewItem{PointID: pointID, Passed: passed}
		if !passed {
			item.IssueType = "insufficient_evidence"
			item.Note = "需要分别补充证据"
		}
		items = append(items, item)
	}
	result, err = service.Review(app.ReviewCommand{
		RequestID:        "reject-two-review-samples",
		JobID:            result.JobID,
		ReviewerID:       "reviewer-b",
		Decision:         "reject",
		Notes:            "两个样本分别退回",
		ExpectedRevision: result.Revision,
		Samples:          sampleResults,
		SampleDigest:     view.Job.ReviewSampleDigest,
		Items:            items,
	})
	mustSucceed(t, err)

	view, err = service.Get(result.JobID)
	if err != nil {
		t.Fatal(err)
	}
	selected := remediationForPoint(t, view, view.Samples[0])
	other := remediationForPoint(t, view, view.Samples[1])
	point := activePoint(t, view, selected.PointID)
	command := app.ReplacePointCommand{
		RequestID:         "replace-one-review-sample",
		JobID:             result.JobID,
		OldPointID:        selected.PointID,
		NewPointID:        selected.PointID + "-replacement",
		ExpectedRevision:  result.Revision,
		PixelX:            point.PixelX,
		PixelY:            point.PixelY,
		MapX:              point.MapX,
		MapY:              point.MapY,
		Reason:            "补充选中样本证据",
		EvidenceNote:      "仅替换选中的退回点",
		ActorID:           "operator-a",
		RemediationItemID: selected.ItemID,
	}
	preview, err := service.PreviewReplacement(command)
	if err != nil {
		t.Fatal(err)
	}
	command.PreviewDigest = preview.PreviewDigest
	if _, err = service.ReplacePoint(command); err != nil {
		t.Fatal(err)
	}

	view, err = service.Get(result.JobID)
	if err != nil {
		t.Fatal(err)
	}
	other = remediationForPoint(t, view, other.PointID)
	if other.ReplacementPointID != "" {
		t.Fatalf("unrelated remediation %s was linked to replacement %s", other.PointID, other.ReplacementPointID)
	}
	for _, instruction := range view.ReturnInstructions {
		if instruction.PointID == other.PointID && instruction.Status != "open" {
			t.Fatalf("unrelated return instruction for %s became %s", other.PointID, instruction.Status)
		}
	}
}

func newService(t *testing.T) *app.Service {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatal(err)
	}
	return app.New(st)
}

func mustSucceed(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func remediationForPoint(t *testing.T, view app.JobView, pointID string) domain.RemediationItem {
	t.Helper()
	for _, item := range view.RemediationItems {
		if item.PointID == pointID && item.Status != "closed" {
			return item
		}
	}
	t.Fatalf("open remediation for %s not found", pointID)
	return domain.RemediationItem{}
}

func activePoint(t *testing.T, view app.JobView, pointID string) domain.ControlPoint {
	t.Helper()
	for _, point := range view.Points {
		if point.PointID == pointID && point.Active {
			return point
		}
	}
	t.Fatalf("active point %s not found", pointID)
	return domain.ControlPoint{}
}

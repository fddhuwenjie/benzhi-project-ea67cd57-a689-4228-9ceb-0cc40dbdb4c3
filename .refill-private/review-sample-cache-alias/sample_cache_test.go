package review_sample_cache_alias_test

import (
	"map-registration-gate/internal/app"
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/store"
	"path/filepath"
	"testing"
)

func TestReviewSampleCacheIsolation(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc := app.New(st)
	sha := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	created, err := svc.CreateJob(app.CreateJobCommand{
		RequestID: "cache-create", JobID: "cache-job", Title: "缓存隔离测试图",
		MapYear: 1936, ImageWidth: 100, ImageHeight: 100, ImageSHA256: sha,
		TargetCRS: "EPSG:3857", RMSELimit: 1, OperatorID: "operator-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := svc.Freeze(app.RevisionCommand{
		RequestID: "cache-freeze", JobID: created.JobID, ExpectedRevision: created.Revision,
	})
	if err != nil {
		t.Fatal(err)
	}

	revision := frozen.Revision
	points := []app.PointCommand{
		{PointID: "north-west", PixelX: 10, PixelY: 10, MapX: 20, MapY: 30},
		{PointID: "north-east", PixelX: 90, PixelY: 10, MapX: 180, MapY: 30},
		{PointID: "south-west", PixelX: 10, PixelY: 90, MapX: 20, MapY: 270},
		{PointID: "south-east", PixelX: 90, PixelY: 90, MapX: 180, MapY: 270},
	}
	for i := range points {
		points[i].RequestID = "cache-point-" + points[i].PointID
		points[i].JobID = created.JobID
		points[i].ExpectedRevision = revision
		points[i].EvidenceNote = "稳定地物证据"
		points[i].ActorID = "operator-a"
		result, addErr := svc.AddPoint(points[i])
		if addErr != nil {
			t.Fatal(addErr)
		}
		revision = result.Revision
	}

	first, err := svc.Get(created.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Samples) == 0 {
		t.Fatal("测试前置条件失败：没有复核样本")
	}
	wantFirstID := first.Samples[0]
	first.Samples[0] = "polluted-by-caller"

	second, err := svc.Get(created.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Samples[0] != wantFirstID {
		t.Fatalf("TestReviewSampleCacheIsolation: 第二次查询复用了已被调用方污染的缓存切片: got %q want %q", second.Samples[0], wantFirstID)
	}
	if second.Job.State != domain.Solvable {
		t.Fatalf("测试任务状态异常: %s", second.Job.State)
	}
}

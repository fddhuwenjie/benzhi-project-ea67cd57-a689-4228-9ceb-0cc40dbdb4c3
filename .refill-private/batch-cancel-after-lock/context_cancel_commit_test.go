package batch_cancel_after_lock_test

import (
	"context"
	"errors"
	"map-registration-gate/internal/app"
	"map-registration-gate/internal/store"
	"path/filepath"
	"sync"
	"testing"
)

type observedContext struct {
	context.Context
	mu      sync.Mutex
	checks  int
	reached chan struct{}
}

func (c *observedContext) Err() error {
	err := c.Context.Err()
	c.mu.Lock()
	c.checks++
	if c.checks == 2 {
		close(c.reached)
	}
	c.mu.Unlock()
	return err
}

func TestCanceledBatchImportDoesNotCommit(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc := app.New(st)
	sha := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	created, err := svc.CreateJob(app.CreateJobCommand{RequestID: "cancel-create", JobID: "cancel-job", Title: "取消测试图", MapYear: 1936, ImageWidth: 100, ImageHeight: 100, ImageSHA256: sha, TargetCRS: "EPSG:3857", RMSELimit: 1, OperatorID: "op"})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := svc.Freeze(app.RevisionCommand{RequestID: "cancel-freeze", JobID: created.JobID, ExpectedRevision: created.Revision})
	if err != nil {
		t.Fatal(err)
	}
	points := []app.BatchPoint{
		{PointID: "a", PixelX: 10, PixelY: 10, MapX: 20, MapY: 30, EvidenceNote: "证据", ActorID: "op"},
		{PointID: "b", PixelX: 90, PixelY: 10, MapX: 180, MapY: 30, EvidenceNote: "证据", ActorID: "op"},
		{PointID: "c", PixelX: 10, PixelY: 90, MapX: 20, MapY: 270, EvidenceNote: "证据", ActorID: "op"},
		{PointID: "d", PixelX: 90, PixelY: 90, MapX: 180, MapY: 270, EvidenceNote: "证据", ActorID: "op"},
	}
	command := app.BatchPointsCommand{RequestID: "cancel-import", JobID: created.JobID, ExpectedRevision: frozen.Revision, Points: points}
	preview, err := svc.PreviewBatch(command)
	if err != nil {
		t.Fatal(err)
	}
	command.PreviewDigest = preview.PreviewDigest
	before, err := svc.Get(created.JobID)
	if err != nil {
		t.Fatal(err)
	}

	transactionEntered := make(chan struct{})
	releaseTransaction := make(chan struct{})
	blockerDone := make(chan error, 1)
	go func() {
		blockerDone <- st.Update(func(data *store.Data) error {
			close(transactionEntered)
			<-releaseTransaction
			return nil
		})
	}()
	<-transactionEntered

	base, cancel := context.WithCancel(context.Background())
	ctx := &observedContext{Context: base, reached: make(chan struct{})}
	importDone := make(chan error, 1)
	go func() {
		_, importErr := svc.ImportBatchContext(ctx, command)
		importDone <- importErr
	}()
	<-ctx.reached
	cancel()
	close(releaseTransaction)
	if err := <-blockerDone; err != nil {
		t.Fatal(err)
	}
	importErr := <-importDone
	view, err := svc.Get(created.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if importErr == nil && len(view.Points) == len(points) {
		t.Fatalf("canceled batch committed %d point(s)", len(view.Points))
	}
	if importErr == nil {
		t.Fatal("canceled batch returned success")
	}
	if !errors.Is(importErr, context.Canceled) {
		t.Fatalf("canceled batch returned wrong error: %v", importErr)
	}
	_, requestRemembered := st.Snapshot().Requests[command.RequestID]
	if len(view.Points) != 0 || view.Job.Revision != frozen.Revision || len(view.Timeline) != len(before.Timeline) || requestRemembered {
		t.Fatalf("canceled batch changed aggregate: points=%d revision=%d events=%d request=%t", len(view.Points), view.Job.Revision, len(view.Timeline), requestRemembered)
	}
}

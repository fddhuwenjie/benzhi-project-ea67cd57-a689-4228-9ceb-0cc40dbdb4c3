package restart_idempotency_response_test

import (
	"path/filepath"
	"testing"

	"map-registration-gate/internal/app"
	"map-registration-gate/internal/store"
)

func TestPersistedIdempotencyReplayTypeRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registration.json")
	firstStore, err := store.Open(path)
	if err != nil {
		t.Fatalf("open first store: %v", err)
	}
	command := app.CreateJobCommand{
		RequestID:   "restart-request",
		JobID:       "restart-job",
		Title:       "重启幂等复现图",
		MapYear:     1936,
		ImageWidth:  1000,
		ImageHeight: 800,
		ImageSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		TargetCRS:   "EPSG:3857",
		RMSELimit:   2.5,
		OperatorID:  "operator-a",
	}
	first, err := app.New(firstStore).CreateJob(command)
	if err != nil {
		t.Fatalf("create before restart: %v", err)
	}

	reopened, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen persisted store: %v", err)
	}
	replayed, err := app.New(reopened).CreateJob(command)
	if err != nil {
		t.Fatalf("replay after restart: %v", err)
	}
	if !replayed.Replayed || replayed.JobID != first.JobID || replayed.Revision != first.Revision || replayed.State != first.State {
		t.Fatalf("persisted replay lost original result: first=%+v replayed=%+v", first, replayed)
	}
}

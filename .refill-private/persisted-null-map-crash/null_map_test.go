package persistednullmapcrash

import (
	"map-registration-gate/internal/app"
	"map-registration-gate/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenNullMapsDoesNotCrashOnCreate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"jobs":null,"points":null,"manifests":null}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	service := app.New(s)
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("panic while creating job after null-map recovery: %v", recovered)
		}
	}()
	if _, err := service.CreateJob(app.CreateJobCommand{
		RequestID:   "null-map-create",
		JobID:       "job-null-map",
		Title:       "恢复测试",
		MapYear:     1930,
		ImageWidth:  1000,
		ImageHeight: 1000,
		ImageSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		TargetCRS:   "EPSG:4490",
		RMSELimit:   2,
		OperatorID:  "operator",
	}); err != nil {
		t.Fatalf("create after null-map recovery: %v", err)
	}
}

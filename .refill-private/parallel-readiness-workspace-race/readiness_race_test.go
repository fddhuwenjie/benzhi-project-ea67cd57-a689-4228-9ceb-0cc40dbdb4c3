package parallel_readiness_workspace_race_test

import (
	"runtime"
	"sync"
	"testing"

	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/georef"
)

func TestParallelReadinessWorkspaceIsolation(t *testing.T) {
	previous := runtime.GOMAXPROCS(2)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })

	jobs := [2]domain.RegistrationJob{
		{JobID: "parallel-job-a", ImageWidth: 100, ImageHeight: 100, Revision: 7},
		{JobID: "parallel-job-b", ImageWidth: 100, ImageHeight: 100, Revision: 11},
	}
	points := []domain.ControlPoint{
		{PointID: "north-west", PixelX: 10, PixelY: 10, Active: true},
		{PointID: "north-east", PixelX: 90, PixelY: 10, Active: true},
		{PointID: "south-west", PixelX: 10, PixelY: 90, Active: true},
		{PointID: "south-east", PixelX: 90, PixelY: 90, Active: true},
	}

	const rounds = 2000
	starts := make([]chan struct{}, rounds)
	for i := range starts {
		starts[i] = make(chan struct{})
	}
	ready := make(chan struct{}, 2)
	errCh := make(chan ReadinessFailure, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	for worker := 0; worker < 2; worker++ {
		go func(job domain.RegistrationJob) {
			defer workers.Done()
			var firstFailure *ReadinessFailure
			for i := 0; i < rounds; i++ {
				ready <- struct{}{}
				<-starts[i]
				diagnosis := georef.Diagnose(job, points)
				if firstFailure == nil && (!diagnosis.Ready || diagnosis.QuadrantCounts != [4]int{1, 1, 1, 1}) {
					firstFailure = &ReadinessFailure{Round: i, Diagnosis: diagnosis}
				}
			}
			if firstFailure != nil {
				errCh <- *firstFailure
			}
		}(jobs[worker])
	}
	for i := 0; i < rounds; i++ {
		<-ready
		<-ready
		close(starts[i])
	}
	workers.Wait()
	close(errCh)
	for failure := range errCh {
		t.Errorf("并行诊断发生状态污染，round=%d counts=%v ready=%v", failure.Round, failure.Diagnosis.QuadrantCounts, failure.Diagnosis.Ready)
	}
}

type ReadinessFailure struct {
	Round     int
	Diagnosis georef.ReadinessDiagnosis
}

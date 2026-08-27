package app

import (
	"fmt"
	"strings"

	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/store"
)

func (s *Service) UpdatePoint(c UpdatePointCommand) (Result, error) {
	if err := validRequest(c.RequestID); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(c.EvidenceNote) == "" {
		return Result{}, fmt.Errorf("%w: revision evidence required", domain.ErrRule)
	}
	unlock := s.lock(c.JobID)
	defer unlock()

	var result Result
	err := s.store.Update(func(data *store.Data) error {
		replayed, found, err := replay(data, c.RequestID, c)
		if err != nil {
			return err
		}
		if found {
			result = replayed
			return nil
		}
		job, found := data.Jobs[c.JobID]
		if !found {
			return domain.ErrNotFound
		}
		if err := checkRevision(job, c.ExpectedRevision); err != nil {
			return err
		}
		if !job.State.AllowsPointMaintenance() || job.State == domain.NeedsFix {
			return domain.ErrInvalidState
		}
		if err := domain.ValidateOperator(job, c.ActorID); err != nil {
			return err
		}

		points := data.Points[c.JobID]
		index := activePointIndex(points, c.PointID)
		if index < 0 {
			return domain.ErrNotFound
		}
		updated := points[index]
		updated.PixelX = c.PixelX
		updated.PixelY = c.PixelY
		updated.MapX = c.MapX
		updated.MapY = c.MapY
		updated.EvidenceNote = c.EvidenceNote
		if err := domain.ValidatePoint(job, points, updated); err != nil {
			return err
		}

		oldHash := store.Hash(points[index])
		points[index] = updated
		data.Points[c.JobID] = points
		invalidateEvaluation(data, c.JobID)
		advancePointRevision(&job, points)
		data.Jobs[c.JobID] = job
		store.AddEvent(data, job.JobID, "point.revised", c.PointID+":"+oldHash)
		result = resultFor(job)
		remember(data, c.RequestID, c, result)
		return nil
	})
	return result, err
}

func (s *Service) DeactivatePoint(c DeactivatePointCommand) (Result, error) {
	if err := validRequest(c.RequestID); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(c.Reason) == "" {
		return Result{}, fmt.Errorf("%w: deactivation reason required", domain.ErrRule)
	}
	unlock := s.lock(c.JobID)
	defer unlock()

	var result Result
	err := s.store.Update(func(data *store.Data) error {
		replayed, found, err := replay(data, c.RequestID, c)
		if err != nil {
			return err
		}
		if found {
			result = replayed
			return nil
		}
		job, found := data.Jobs[c.JobID]
		if !found {
			return domain.ErrNotFound
		}
		if err := checkRevision(job, c.ExpectedRevision); err != nil {
			return err
		}
		if !job.State.AllowsPointMaintenance() || job.State == domain.NeedsFix {
			return domain.ErrInvalidState
		}
		if err := domain.ValidateOperator(job, c.ActorID); err != nil {
			return err
		}

		points := data.Points[c.JobID]
		index := activePointIndex(points, c.PointID)
		if index < 0 {
			return domain.ErrNotFound
		}
		points[index].Active = false
		data.Points[c.JobID] = points
		invalidateEvaluation(data, c.JobID)
		advancePointRevision(&job, points)
		data.Jobs[c.JobID] = job
		store.AddEvent(data, job.JobID, "point.deactivated", c.PointID+":"+c.Reason)
		result = resultFor(job)
		remember(data, c.RequestID, c, result)
		return nil
	})
	return result, err
}

func activePointIndex(points []domain.ControlPoint, pointID string) int {
	for index := range points {
		if points[index].PointID == pointID && points[index].Active {
			return index
		}
	}
	return -1
}

func invalidateEvaluation(data *store.Data, jobID string) {
	job := data.Jobs[jobID]
	job.CurrentEvaluationID = ""
	job.ReviewSampleDigest = ""
	data.Jobs[jobID] = job
	delete(data.Reviews, jobID)
}

func advancePointRevision(job *domain.RegistrationJob, points []domain.ControlPoint) {
	job.Revision++
	job.CurrentEvaluationID = ""
	job.ReviewSampleDigest = ""
	if domain.Distribution(*job, points) {
		job.State = domain.Solvable
		return
	}
	job.State = domain.Frozen
}

func resultFor(job domain.RegistrationJob) Result {
	return Result{JobID: job.JobID, Revision: job.Revision, State: string(job.State)}
}

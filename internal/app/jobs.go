package app

import (
	"fmt"
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/store"
)

func (s *Service) CreateJob(c CreateJobCommand) (Result, error) {
	if err := validRequest(c.RequestID); err != nil {
		return Result{}, err
	}
	if c.JobID == "" {
		c.JobID = stableID("job", c.RequestID)
	}
	prototype := domain.RegistrationJob{Title: c.Title, MapYear: c.MapYear, ImageWidth: c.ImageWidth, ImageHeight: c.ImageHeight, ImageSHA256: c.ImageSHA256, TargetCRS: c.TargetCRS, RMSELimit: c.RMSELimit, OperatorID: c.OperatorID}
	if e := domain.ValidateBaseline(prototype); e != nil {
		return Result{}, e
	}
	unlock := s.lock(c.JobID)
	defer unlock()
	var out Result
	err := s.store.Update(func(d *store.Data) error {
		if r, ok, e := replay(d, c.RequestID, c); ok || e != nil {
			out = r
			return e
		}
		if _, ok := d.Jobs[c.JobID]; ok {
			return domain.ErrConflict
		}
		j := domain.RegistrationJob{JobID: c.JobID, Title: c.Title, MapYear: c.MapYear, ImageWidth: c.ImageWidth, ImageHeight: c.ImageHeight, ImageSHA256: c.ImageSHA256, TargetCRS: c.TargetCRS, RMSELimit: c.RMSELimit, State: domain.Draft, Revision: 1, OperatorID: c.OperatorID, CreatedAt: s.now()}
		d.Jobs[j.JobID] = j
		store.AddEvent(d, j.JobID, "job.created", j.Title)
		out = Result{j.JobID, j.Revision, string(j.State), false}
		remember(d, c.RequestID, c, out)
		return nil
	})
	return out, err
}
func (s *Service) Freeze(c RevisionCommand) (Result, error) {
	return s.revise(c, "baseline.frozen", func(j *domain.RegistrationJob) error {
		if err := domain.ValidateBaseline(*j); err != nil {
			return err
		}
		return j.Freeze()
	})
}

type BaselineConfirmation struct {
	JobID       string  `json:"job_id"`
	Title       string  `json:"title"`
	ImageSHA256 string  `json:"image_sha256"`
	TargetCRS   string  `json:"target_crs"`
	Digest      string  `json:"digest"`
	MapYear     int     `json:"map_year"`
	ImageWidth  int     `json:"image_width"`
	ImageHeight int     `json:"image_height"`
	RMSELimit   float64 `json:"rmse_limit"`
	Revision    uint64  `json:"revision"`
}

func (s *Service) BaselineConfirmation(jobID string) (BaselineConfirmation, error) {
	d := s.store.Snapshot()
	j, ok := d.Jobs[jobID]
	if !ok {
		return BaselineConfirmation{}, domain.ErrNotFound
	}
	if err := domain.ValidateBaseline(j); err != nil {
		return BaselineConfirmation{}, err
	}
	return BaselineConfirmation{j.JobID, j.Title, j.ImageSHA256, j.TargetCRS, domain.BaselineDigest(j), j.MapYear, j.ImageWidth, j.ImageHeight, j.RMSELimit, j.Revision}, nil
}

func (s *Service) ReviseBaseline(c ReviseBaselineCommand) (Result, error) {
	if err := validRequest(c.RequestID); err != nil {
		return Result{}, err
	}
	unlock := s.lock(c.JobID)
	defer unlock()
	var out Result
	err := s.store.Update(func(d *store.Data) error {
		if r, ok, err := replay(d, c.RequestID, c); ok || err != nil {
			out = r
			return err
		}
		j, ok := d.Jobs[c.JobID]
		if !ok {
			return domain.ErrNotFound
		}
		if j.State != domain.Draft {
			return domain.ErrImmutable
		}
		if err := checkRevision(j, c.ExpectedRevision); err != nil {
			return err
		}
		before := domain.BaselineDigest(j)
		j.Title, j.MapYear = c.Title, c.MapYear
		j.ImageWidth, j.ImageHeight = c.ImageWidth, c.ImageHeight
		j.ImageSHA256, j.TargetCRS, j.RMSELimit = c.ImageSHA256, c.TargetCRS, c.RMSELimit
		if err := domain.ValidateBaseline(j); err != nil {
			return err
		}
		j.Revision++
		d.Jobs[j.JobID] = j
		store.AddEvent(d, j.JobID, "baseline.revised", before+":"+domain.BaselineDigest(j))
		out = resultFor(j)
		remember(d, c.RequestID, c, out)
		return nil
	})
	return out, err
}
func (s *Service) SubmitReview(c RevisionCommand) (Result, error) {
	if e := validRequest(c.RequestID); e != nil {
		return Result{}, e
	}
	unlock := s.lock(c.JobID)
	defer unlock()
	var out Result
	err := s.store.Update(func(d *store.Data) error {
		if r, ok, e := replay(d, c.RequestID, c); ok || e != nil {
			out = r
			return e
		}
		j, ok := d.Jobs[c.JobID]
		if !ok {
			return domain.ErrNotFound
		}
		if e := checkRevision(j, c.ExpectedRevision); e != nil {
			return e
		}
		ev, ok := d.Evals[c.JobID]
		if !ok || ev.Decision != "pass" || j.CurrentEvaluationID != ev.EvaluationID || ev.PointSetDigest != domain.ControlSetDigest(d.Points[c.JobID]) {
			return fmt.Errorf("%w: current points require a passing evaluation", domain.ErrRule)
		}
		for _, item := range d.RemediationItems[c.JobID] {
			if item.Status != "closed" {
				return fmt.Errorf("%w: open remediation items remain", domain.ErrRule)
			}
		}
		if e := j.SubmitReview(); e != nil {
			return e
		}
		j.ReviewSampleDigest = reviewSampleDigest(d.Points[c.JobID], j.ImageSHA256)
		d.Jobs[c.JobID] = j
		store.AddEvent(d, j.JobID, "review.submitted", ev.EvaluationID)
		out = Result{j.JobID, j.Revision, string(j.State), false}
		remember(d, c.RequestID, c, out)
		return nil
	})
	return out, err
}
func (s *Service) revise(c RevisionCommand, event string, fn func(*domain.RegistrationJob) error) (Result, error) {
	if e := validRequest(c.RequestID); e != nil {
		return Result{}, e
	}
	unlock := s.lock(c.JobID)
	defer unlock()
	var out Result
	err := s.store.Update(func(d *store.Data) error {
		if r, ok, e := replay(d, c.RequestID, c); ok || e != nil {
			out = r
			return e
		}
		j, ok := d.Jobs[c.JobID]
		if !ok {
			return domain.ErrNotFound
		}
		if e := checkRevision(j, c.ExpectedRevision); e != nil {
			return e
		}
		if e := fn(&j); e != nil {
			return e
		}
		d.Jobs[c.JobID] = j
		store.AddEvent(d, j.JobID, event, "")
		out = Result{j.JobID, j.Revision, string(j.State), false}
		remember(d, c.RequestID, c, out)
		return nil
	})
	return out, err
}

package app

import (
	"fmt"
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/georef"
	"map-registration-gate/internal/store"
	"strings"
)

func (s *Service) Review(c ReviewCommand) (Result, error) {
	if e := validRequest(c.RequestID); e != nil {
		return Result{}, e
	}
	if c.Decision != "approve" && c.Decision != "reject" {
		return Result{}, fmt.Errorf("%w: review decision", domain.ErrRule)
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
		if e := domain.ValidateReviewer(j, c.ReviewerID); e != nil {
			return e
		}
		samples := georef.Sample(d.Points[c.JobID], j.ImageSHA256)
		expectedDigest := reviewSampleDigest(d.Points[c.JobID], j.ImageSHA256)
		if j.ReviewSampleDigest == "" || j.ReviewSampleDigest != expectedDigest || c.SampleDigest != expectedDigest {
			return fmt.Errorf("%w: review sample set expired", domain.ErrConflict)
		}
		if len(c.Samples) != len(samples) {
			return fmt.Errorf("%w: review sample set mismatch", domain.ErrRule)
		}
		for _, id := range samples {
			if _, ok := c.Samples[id]; !ok {
				return fmt.Errorf("%w: sample conclusion missing", domain.ErrRule)
			}
		}
		items := append([]domain.ReviewItem(nil), c.Items...)
		if len(items) == 0 && c.Decision == "approve" {
			for _, id := range samples {
				items = append(items, domain.ReviewItem{PointID: id, Passed: true})
			}
		}
		if len(items) != len(samples) {
			return fmt.Errorf("%w: structured sample conclusions required", domain.ErrRule)
		}
		seen, failed := map[string]bool{}, false
		for _, item := range items {
			if seen[item.PointID] {
				return fmt.Errorf("%w: duplicate sample conclusion", domain.ErrRule)
			}
			seen[item.PointID] = true
			passed, ok := c.Samples[item.PointID]
			if !ok || passed != item.Passed {
				return fmt.Errorf("%w: sample conclusion mismatch", domain.ErrRule)
			}
			if !item.Passed {
				failed = true
				if !validReviewIssue(item.IssueType) {
					return fmt.Errorf("%w: failed sample issue type required", domain.ErrRule)
				}
				if strings.TrimSpace(item.Note) == "" {
					return fmt.Errorf("%w: failed sample note required", domain.ErrRule)
				}
			}
		}
		if c.Decision == "approve" {
			if failed {
				return fmt.Errorf("%w: failed sample cannot approve", domain.ErrRule)
			}
			if e := j.AcceptReview(); e != nil {
				return e
			}
		} else {
			if !failed {
				return fmt.Errorf("%w: rejection requires a failed return item", domain.ErrRule)
			}
			if e := j.RejectReview(); e != nil {
				return e
			}
			for _, item := range items {
				if !item.Passed {
					instructionID := stableID("instruction", c.RequestID+":"+item.PointID)
					d.ReturnInstructions[j.JobID] = append(d.ReturnInstructions[j.JobID], domain.ReturnInstruction{InstructionID: instructionID, JobID: j.JobID, PointID: item.PointID, IssueType: item.IssueType, Note: item.Note, ReviewSampleDigest: expectedDigest, Status: "open", CreatedAt: s.now()})
					residual := d.Evals[j.JobID].PointResiduals[item.PointID]
					d.RemediationItems[j.JobID] = append(d.RemediationItems[j.JobID], domain.RemediationItem{ItemID: "item-" + requestHash(struct{ InstructionID string }{instructionID})[:16], JobID: j.JobID, PointID: item.PointID, EvaluationID: j.CurrentEvaluationID, RuleSource: "review_" + item.IssueType, TriggerResidual: residual, LatestResidual: residual, Status: "open", CreatedAt: s.now(), UpdatedAt: s.now()})
				}
			}
		}
		d.Reviews[j.JobID] = domain.Review{ReviewerID: c.ReviewerID, Samples: c.Samples, SampleDigest: expectedDigest, Items: items, Notes: c.Notes, Decision: c.Decision, CreatedAt: s.now()}
		d.Jobs[j.JobID] = j
		store.AddEvent(d, j.JobID, "review."+c.Decision, c.Notes)
		out = Result{j.JobID, j.Revision, string(j.State), false}
		remember(d, c.RequestID, c, out)
		return nil
	})
	return out, err
}

func reviewSampleDigest(points []domain.ControlPoint, seed string) string {
	return domain.Digest(struct {
		PointSetDigest string
		Samples        []string
	}{domain.ControlSetDigest(points), georef.Sample(points, seed)})
}
func validReviewIssue(issue string) bool {
	return issue == "coordinate_deviation" || issue == "insufficient_evidence" || issue == "overlay_anomaly"
}

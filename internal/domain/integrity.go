package domain

import (
	"fmt"
	"math"
)

type AggregateEvidence struct {
	Job          RegistrationJob
	Points       []ControlPoint
	Evaluation   *FitEvaluation
	Review       *Review
	Manifest     *ReleaseManifest
	Remediations []Remediation
}

func ValidateAggregate(e AggregateEvidence) error {
	if e.Job.JobID == "" || e.Job.Revision == 0 || !e.Job.State.Known() {
		return fmt.Errorf("%w: malformed job aggregate", ErrRule)
	}
	if err := ValidateBaseline(e.Job); err != nil {
		return err
	}
	pointIDs := map[string]bool{}
	activePixels := map[string]string{}
	for _, p := range e.Points {
		if p.PointID == "" || p.JobID != e.Job.JobID || p.CreatedBy == "" {
			return fmt.Errorf("%w: malformed control point", ErrRule)
		}
		if pointIDs[p.PointID] {
			return fmt.Errorf("%w: duplicate point identity", ErrRule)
		}
		pointIDs[p.PointID] = true
		if !finite(p.PixelX, p.PixelY, p.MapX, p.MapY) || p.PixelX < 0 || p.PixelY < 0 || p.PixelX > float64(e.Job.ImageWidth) || p.PixelY > float64(e.Job.ImageHeight) {
			return fmt.Errorf("%w: invalid point coordinates", ErrRule)
		}
		if p.Active {
			key := fmt.Sprintf("%g/%g", p.PixelX, p.PixelY)
			if prior := activePixels[key]; prior != "" {
				return fmt.Errorf("%w: active pixel duplicate %s and %s", ErrRule, prior, p.PointID)
			}
			activePixels[key] = p.PointID
		}
		if p.SupersedesPointID != "" && !pointIDs[p.SupersedesPointID] {
			found := false
			for _, candidate := range e.Points {
				if candidate.PointID == p.SupersedesPointID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("%w: superseded point missing", ErrRule)
			}
		}
	}
	if e.Job.State == Draft && len(e.Points) > 0 {
		return fmt.Errorf("%w: draft contains points", ErrRule)
	}
	if e.Job.State == Solvable && !Distribution(e.Job, e.Points) {
		return fmt.Errorf("%w: solvable state lacks coverage", ErrRule)
	}
	if e.Evaluation != nil {
		if e.Evaluation.JobID != e.Job.JobID || e.Evaluation.EvaluationID == "" || !finite(e.Evaluation.RMSE, e.Evaluation.MaxResidual) {
			return fmt.Errorf("%w: malformed evaluation", ErrRule)
		}
		for _, c := range e.Evaluation.Coefficients {
			if math.IsNaN(c) || math.IsInf(c, 0) {
				return fmt.Errorf("%w: non-finite coefficient", ErrRule)
			}
		}
		for id, r := range e.Evaluation.PointResiduals {
			if !pointIDs[id] || math.IsNaN(r) || math.IsInf(r, 0) || r < 0 {
				return fmt.Errorf("%w: malformed residual", ErrRule)
			}
		}
		if e.Evaluation.PointSetDigest != "" && len(e.Evaluation.PointSetDigest) != 64 {
			return fmt.Errorf("%w: malformed point set digest", ErrRule)
		}
	}
	if e.Review != nil {
		if err := ValidateReviewer(e.Job, e.Review.ReviewerID); err != nil {
			return err
		}
		if e.Review.Decision != "approve" && e.Review.Decision != "reject" {
			return fmt.Errorf("%w: review decision", ErrRule)
		}
	}
	for _, r := range e.Remediations {
		if r.JobID != e.Job.JobID || r.OldPointID == "" || r.NewPointID == "" || r.Reason == "" || r.ReplacementEvidence == "" || r.ActorID != e.Job.OperatorID {
			return fmt.Errorf("%w: malformed remediation", ErrRule)
		}
		if !pointIDs[r.OldPointID] || !pointIDs[r.NewPointID] {
			return fmt.Errorf("%w: remediation point missing", ErrRule)
		}
	}
	if e.Job.State == Published {
		if e.Manifest == nil || e.Manifest.JobID != e.Job.JobID || e.Manifest.CanonicalSHA256 == "" {
			return fmt.Errorf("%w: published job lacks manifest", ErrRule)
		}
	} else if e.Manifest != nil {
		return fmt.Errorf("%w: manifest exists before publication", ErrRule)
	}
	return nil
}

package app

import (
	"fmt"
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/georef"
	"sort"
)

func buildReplacementPreview(j domain.RegistrationJob, points []domain.ControlPoint, current domain.FitEvaluation, c ReplacePointCommand) (ReplacementPreview, error) {
	if c.Reason == "" || c.EvidenceNote == "" {
		return ReplacementPreview{}, fmt.Errorf("%w: remediation evidence required", domain.ErrRule)
	}
	if current.EvaluationID == "" || j.CurrentEvaluationID != current.EvaluationID {
		return ReplacementPreview{}, fmt.Errorf("%w: current evaluation required", domain.ErrRule)
	}
	temp := append([]domain.ControlPoint(nil), points...)
	found := false
	for i := range temp {
		if temp[i].PointID == c.OldPointID && temp[i].Active {
			temp[i].Active = false
			found = true
		}
	}
	if !found {
		return ReplacementPreview{}, domain.ErrNotFound
	}
	for _, p := range temp {
		if p.PointID == c.NewPointID {
			return ReplacementPreview{}, fmt.Errorf("%w: duplicate point_id", domain.ErrRule)
		}
	}
	candidate := domain.ControlPoint{PointID: c.NewPointID, JobID: j.JobID, PixelX: c.PixelX, PixelY: c.PixelY, MapX: c.MapX, MapY: c.MapY, EvidenceNote: c.EvidenceNote, CreatedBy: c.ActorID, Active: true, SupersedesPointID: c.OldPointID}
	if err := domain.ValidatePoint(j, temp, candidate); err != nil {
		return ReplacementPreview{}, err
	}
	temp = append(temp, candidate)
	diagnosis := georef.Diagnose(j, temp)
	out := ReplacementPreview{BeforeRMSE: current.RMSE, BeforeMaxResidual: current.MaxResidual, BeforeQuadrants: georef.Diagnose(j, points).QuadrantCounts, AfterQuadrants: diagnosis.QuadrantCounts, ResidualChanges: map[string]float64{}}
	if !diagnosis.Ready {
		for _, r := range diagnosis.Rules {
			if r.Blocking {
				out.RuleFailures = append(out.RuleFailures, r.Code)
			}
		}
		sort.Strings(out.RuleFailures)
	} else {
		fit, err := georef.Fit(j, temp)
		if err != nil {
			return out, fmt.Errorf("%w: %v", domain.ErrRule, err)
		}
		quality := georef.Assess(fit, georef.DefaultQualityRules(j))
		out.AfterRMSE, out.AfterMaxResidual = fit.RMSE, fit.Max
		out.RuleFailures = append(out.RuleFailures, quality.Failures...)
		out.WouldPass = quality.Passed
		for id, after := range fit.Residuals {
			out.ResidualChanges[id] = after - current.PointResiduals[id]
		}
	}
	out.PreviewDigest = requestHash(struct {
		JobID, OldID, NewID        string
		Revision                   uint64
		PixelX, PixelY, MapX, MapY float64
		Reason, Evidence, Actor    string
	}{j.JobID, c.OldPointID, c.NewPointID, j.Revision, c.PixelX, c.PixelY, c.MapX, c.MapY, c.Reason, c.EvidenceNote, c.ActorID})
	return out, nil
}

package app

import (
	"fmt"
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/georef"
	"map-registration-gate/internal/store"
	"time"
)

func (s *Service) Evaluate(c RevisionCommand) (Result, error) {
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
		if j.State != domain.Solvable && j.State != domain.NeedsFix {
			return domain.ErrInvalidState
		}
		diagnosis := georef.Diagnose(j, d.Points[c.JobID])
		if !diagnosis.Ready {
			return fmt.Errorf("%w: point set readiness diagnostics contain blocking rules", domain.ErrRule)
		}
		fit, e := georef.Fit(j, d.Points[c.JobID])
		if e != nil {
			return e
		}
		assessment := georef.Assess(fit, georef.DefaultQualityRules(j))
		decision := "pass"
		j.State = domain.Solvable
		if !assessment.Passed {
			decision = "remediate"
			j.State = domain.NeedsFix
		}
		j.Revision++
		ev := domain.FitEvaluation{EvaluationID: stableID("eval", c.RequestID), JobID: j.JobID, InputRevision: c.ExpectedRevision, PointSetDigest: domain.ControlSetDigest(d.Points[j.JobID]), Coefficients: fit.Coefficients, PointResiduals: fit.Residuals, RMSE: fit.RMSE, MaxResidual: fit.Max, DistributionPassed: fit.Distribution, Decision: decision, RuleFailures: assessment.Failures, OutlierPointIDs: assessment.OutlierPointIDs, EvaluatedAt: s.now()}
		j.CurrentEvaluationID = ev.EvaluationID
		d.Evals[j.JobID] = ev
		d.EvalHistory[j.JobID] = append(d.EvalHistory[j.JobID], ev)
		updateRemediationItems(d, j, ev, s.now())
		d.Jobs[j.JobID] = j
		store.AddEvent(d, j.JobID, "evaluation."+decision, store.Hash(ev))
		out = Result{j.JobID, j.Revision, string(j.State), false}
		remember(d, c.RequestID, c, out)
		return nil
	})
	return out, err
}

func updateRemediationItems(d *store.Data, j domain.RegistrationJob, ev domain.FitEvaluation, now time.Time) {
	items := d.RemediationItems[j.JobID]
	open := map[string]int{}
	for i := range items {
		if items[i].Status != "closed" {
			open[items[i].PointID] = i
		}
	}
	outliers := map[string]bool{}
	for _, id := range ev.OutlierPointIDs {
		outliers[id] = true
		if i, ok := open[id]; ok {
			items[i].LatestResidual = ev.PointResiduals[id]
			items[i].Status = "reopened"
			items[i].UpdatedAt = now
			continue
		}
		items = append(items, domain.RemediationItem{ItemID: stableItemID(j.JobID, id), JobID: j.JobID, PointID: id, EvaluationID: ev.EvaluationID, RuleSource: "point_residual", TriggerResidual: ev.PointResiduals[id], LatestResidual: ev.PointResiduals[id], Status: "open", CreatedAt: now, UpdatedAt: now})
	}
	for i := range items {
		if items[i].Status == "closed" || outliers[items[i].PointID] {
			continue
		}
		if items[i].ReplacementPointID != "" {
			if residual, ok := ev.PointResiduals[items[i].ReplacementPointID]; ok && residual <= j.RMSELimit*2 {
				items[i].LatestResidual = residual
				items[i].Status = "closed"
				items[i].UpdatedAt = now
			}
		}
	}
	d.RemediationItems[j.JobID] = items
}

func stableItemID(jobID, pointID string) string {
	return "item-" + requestHash(struct{ JobID, PointID string }{jobID, pointID})[:16]
}

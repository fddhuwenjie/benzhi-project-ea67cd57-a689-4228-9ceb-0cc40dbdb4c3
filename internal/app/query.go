package app

import (
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/georef"
	"sort"
)

type JobView struct {
	Job                domain.RegistrationJob     `json:"job"`
	Points             []domain.ControlPoint      `json:"points"`
	Evaluation         *domain.FitEvaluation      `json:"evaluation,omitempty"`
	Review             *domain.Review             `json:"review,omitempty"`
	Manifest           *domain.ReleaseManifest    `json:"manifest,omitempty"`
	Samples            []string                   `json:"samples"`
	Timeline           []domain.AuditEvent        `json:"timeline"`
	Remediations       []domain.Remediation       `json:"remediations"`
	RemediationItems   []domain.RemediationItem   `json:"remediation_items"`
	ReturnInstructions []domain.ReturnInstruction `json:"return_instructions"`
	Readiness          georef.ReadinessDiagnosis  `json:"readiness"`
	EvaluationHistory  []EvaluationHistoryView    `json:"evaluation_history"`
	Workflow           WorkflowProjection         `json:"workflow"`
}
type EvaluationHistoryView struct {
	Evaluation  domain.FitEvaluation         `json:"evaluation"`
	Current     bool                         `json:"current"`
	Invalidated bool                         `json:"invalidated"`
	Comparison  *domain.EvaluationComparison `json:"comparison,omitempty"`
}

func (s *Service) Get(jobID string) (JobView, error) {
	d := s.store.Snapshot()
	j, ok := d.Jobs[jobID]
	if !ok {
		return JobView{}, domain.ErrNotFound
	}
	v := JobView{Job: j, Points: append([]domain.ControlPoint(nil), d.Points[jobID]...)}
	v.Remediations = append([]domain.Remediation(nil), d.Remediations[jobID]...)
	v.RemediationItems = append([]domain.RemediationItem(nil), d.RemediationItems[jobID]...)
	v.ReturnInstructions = append([]domain.ReturnInstruction(nil), d.ReturnInstructions[jobID]...)
	v.Readiness = georef.Diagnose(j, v.Points)
	if e, ok := d.Evals[jobID]; ok && j.CurrentEvaluationID == e.EvaluationID {
		v.Evaluation = &e
	}
	if r, ok := d.Reviews[jobID]; ok {
		v.Review = &r
	}
	if m, ok := d.Manifests[jobID]; ok {
		v.Manifest = &m
	}
	v.Samples = georef.Sample(v.Points, j.ImageSHA256)
	v.EvaluationHistory = historyViews(d.EvalHistory[jobID], j.CurrentEvaluationID)
	v.Workflow = projectWorkflow(j, v.Points, v.Evaluation, v.Manifest, v.Readiness, v.RemediationItems)
	for _, e := range d.Events {
		if e.JobID == jobID {
			v.Timeline = append(v.Timeline, e)
		}
	}
	return v, nil
}

func historyViews(history []domain.FitEvaluation, current string) []EvaluationHistoryView {
	h := append([]domain.FitEvaluation(nil), history...)
	sort.Slice(h, func(i, k int) bool {
		if h[i].EvaluatedAt.Equal(h[k].EvaluatedAt) {
			return h[i].EvaluationID < h[k].EvaluationID
		}
		return h[i].EvaluatedAt.Before(h[k].EvaluatedAt)
	})
	out := make([]EvaluationHistoryView, len(h))
	for i, e := range h {
		out[i] = EvaluationHistoryView{Evaluation: e, Current: e.EvaluationID == current, Invalidated: e.EvaluationID != current}
		if i > 0 {
			c := compareEvaluations(h[i-1], e)
			out[i].Comparison = &c
		}
	}
	return out
}
func compareEvaluations(a, b domain.FitEvaluation) domain.EvaluationComparison {
	c := domain.EvaluationComparison{FromEvaluationID: a.EvaluationID, ToEvaluationID: b.EvaluationID, RMSEChange: b.RMSE - a.RMSE, MaxResidualChange: b.MaxResidual - a.MaxResidual, OutlierCountChange: len(b.OutlierPointIDs) - len(a.OutlierPointIDs), Trend: "unchanged"}
	for i := range c.CoefficientChanges {
		c.CoefficientChanges[i] = b.Coefficients[i] - a.Coefficients[i]
	}
	if c.RMSEChange < -1e-12 {
		c.Trend = "improved"
	} else if c.RMSEChange > 1e-12 {
		c.Trend = "worsened"
	}
	return c
}
func (s *Service) List() []domain.RegistrationJob {
	d := s.store.Snapshot()
	out := make([]domain.RegistrationJob, 0, len(d.Jobs))
	for _, j := range d.Jobs {
		out = append(out, j)
	}
	sort.Slice(out, func(i, k int) bool { return out[i].CreatedAt.Before(out[k].CreatedAt) })
	return out
}

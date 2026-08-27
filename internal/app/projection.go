package app

import (
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/georef"
)

type AvailableActions struct {
	Freeze         bool `json:"freeze"`
	AddPoint       bool `json:"add_point"`
	RevisePoint    bool `json:"revise_point"`
	Remediate      bool `json:"remediate"`
	Evaluate       bool `json:"evaluate"`
	SubmitReview   bool `json:"submit_review"`
	Review         bool `json:"review"`
	Release        bool `json:"release"`
	VerifyManifest bool `json:"verify_manifest"`
	ReviseBaseline bool `json:"revise_baseline"`
	BatchImport    bool `json:"batch_import"`
}
type PointSetProjection struct {
	Total              int  `json:"total"`
	Active             int  `json:"active"`
	Inactive           int  `json:"inactive"`
	Quadrants          int  `json:"quadrants"`
	DistributionPassed bool `json:"distribution_passed"`
}
type QualityProjection struct {
	Evaluated       bool     `json:"evaluated"`
	Passed          bool     `json:"passed"`
	RMSE            float64  `json:"rmse"`
	MaxResidual     float64  `json:"max_residual"`
	RuleFailures    []string `json:"rule_failures"`
	OutlierPointIDs []string `json:"outlier_point_ids"`
}
type WorkflowProjection struct {
	StateLabel string             `json:"state_label"`
	Points     PointSetProjection `json:"points"`
	Quality    QualityProjection  `json:"quality"`
	Actions    AvailableActions   `json:"actions"`
}

func projectWorkflow(j domain.RegistrationJob, p []domain.ControlPoint, e *domain.FitEvaluation, m *domain.ReleaseManifest, diagnosis georef.ReadinessDiagnosis, items []domain.RemediationItem) WorkflowProjection {
	ps := projectPointSet(j, p)
	q := QualityProjection{}
	if e != nil {
		q = QualityProjection{true, e.Decision == "pass", e.RMSE, e.MaxResidual, e.RuleFailures, e.OutlierPointIDs}
	}
	openItems := false
	for _, item := range items {
		if item.Status != "closed" {
			openItems = true
		}
	}
	a := AvailableActions{Freeze: j.State == domain.Draft, ReviseBaseline: j.State == domain.Draft, BatchImport: j.State == domain.Frozen || j.State == domain.Solvable, AddPoint: j.State == domain.Frozen || j.State == domain.Solvable, RevisePoint: j.State == domain.Frozen || j.State == domain.Solvable, Remediate: j.State == domain.NeedsFix, Evaluate: (j.State == domain.Solvable || j.State == domain.NeedsFix) && diagnosis.Ready, SubmitReview: j.State == domain.Solvable && q.Passed && !openItems, Review: j.State == domain.PendingReview, Release: j.State == domain.PendingRelease, VerifyManifest: j.State == domain.Published && m != nil}
	return WorkflowProjection{stateLabel(j.State), ps, q, a}
}
func projectPointSet(j domain.RegistrationJob, p []domain.ControlPoint) PointSetProjection {
	ps := PointSetProjection{Total: len(p)}
	q := map[int]bool{}
	for _, x := range p {
		if !x.Active {
			ps.Inactive++
			continue
		}
		ps.Active++
		z := 0
		if x.PixelX >= float64(j.ImageWidth)/2 {
			z++
		}
		if x.PixelY >= float64(j.ImageHeight)/2 {
			z += 2
		}
		q[z] = true
	}
	ps.Quadrants = len(q)
	ps.DistributionPassed = ps.Active >= 4 && ps.Quadrants == 4
	return ps
}
func stateLabel(s domain.State) string {
	switch s {
	case domain.Draft:
		return "草稿"
	case domain.Frozen:
		return "基线已冻结"
	case domain.Solvable:
		return "可求解"
	case domain.NeedsFix:
		return "待整改"
	case domain.PendingReview:
		return "待复核"
	case domain.PendingRelease:
		return "待发布"
	case domain.Published:
		return "已发布"
	default:
		return "未知状态"
	}
}

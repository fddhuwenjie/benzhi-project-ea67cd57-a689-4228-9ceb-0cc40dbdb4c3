package georef

import (
	"map-registration-gate/internal/domain"
	"sort"
)

type QualityRules struct {
	RMSELimit           float64
	PointResidualLimit  float64
	RequireDistribution bool
}
type QualityAssessment struct {
	Passed          bool
	Failures        []string
	OutlierPointIDs []string
}

func Assess(result Result, rules QualityRules) QualityAssessment {
	a := QualityAssessment{Passed: true}
	if rules.RequireDistribution && !result.Distribution {
		a.Passed = false
		a.Failures = append(a.Failures, "distribution")
	}
	if result.RMSE > rules.RMSELimit {
		a.Passed = false
		a.Failures = append(a.Failures, "rmse")
	}
	ids := make([]string, 0)
	for id, residual := range result.Residuals {
		if residual > rules.PointResidualLimit {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > 0 {
		a.Passed = false
		a.Failures = append(a.Failures, "point_residual")
		a.OutlierPointIDs = ids
	}
	return a
}

func DefaultQualityRules(job domain.RegistrationJob) QualityRules {
	return QualityRules{RMSELimit: job.RMSELimit, PointResidualLimit: job.RMSELimit * 2, RequireDistribution: true}
}

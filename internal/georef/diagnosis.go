package georef

import (
	"map-registration-gate/internal/domain"
	"math"
	"sort"
)

type DiagnosticRule struct {
	Code     string   `json:"code"`
	Reason   string   `json:"reason"`
	PointIDs []string `json:"point_ids"`
	Blocking bool     `json:"blocking"`
}

type ReadinessDiagnosis struct {
	Revision         uint64           `json:"revision"`
	Active           int              `json:"active"`
	Inactive         int              `json:"inactive"`
	MinimumRequired  int              `json:"minimum_required"`
	MinimumShortfall int              `json:"minimum_shortfall"`
	QuadrantCounts   [4]int           `json:"quadrant_counts"`
	MissingQuadrants []int            `json:"missing_quadrants"`
	Ready            bool             `json:"ready"`
	Rules            []DiagnosticRule `json:"rules"`
}

func Diagnose(job domain.RegistrationJob, points []domain.ControlPoint) ReadinessDiagnosis {
	d := ReadinessDiagnosis{Revision: job.Revision, MinimumRequired: 4}
	active := domain.ActivePoints(points)
	d.Active = len(active)
	d.Inactive = len(points) - len(active)
	if d.Active < d.MinimumRequired {
		d.MinimumShortfall = d.MinimumRequired - d.Active
		d.Rules = append(d.Rules, DiagnosticRule{"minimum_count", "有效控制点数量不足", nil, true})
	}
	quadrantWorkspace := [4]int{}
	for _, p := range active {
		q := quadrant(job, p)
		quadrantWorkspace[q]++
	}
	d.QuadrantCounts = quadrantWorkspace
	for q, n := range d.QuadrantCounts {
		if n == 0 {
			d.MissingQuadrants = append(d.MissingQuadrants, q+1)
		}
	}
	if len(d.MissingQuadrants) > 0 {
		d.Rules = append(d.Rules, DiagnosticRule{"missing_quadrant", "存在未覆盖象限，需要在对应区域补点", nil, true})
	}
	boundary := make([]string, 0)
	marginX, marginY := float64(job.ImageWidth)*0.01, float64(job.ImageHeight)*0.01
	for _, p := range active {
		if p.PixelX <= marginX || p.PixelX >= float64(job.ImageWidth)-marginX || p.PixelY <= marginY || p.PixelY >= float64(job.ImageHeight)-marginY {
			boundary = append(boundary, p.PointID)
		}
	}
	if len(boundary) > 0 {
		d.Rules = append(d.Rules, DiagnosticRule{"near_boundary", "部分点位临近图幅边界，请核对定位精度", boundary, false})
	}
	clustered := clusteredIDs(job, active)
	if len(clustered) > 0 {
		d.Rules = append(d.Rules, DiagnosticRule{"pixel_cluster", "部分像素位置过度聚集", clustered, false})
	}
	if len(active) >= 4 && degenerate(job, active) {
		ids := make([]string, len(active))
		for i := range active {
			ids[i] = active[i].PointID
		}
		d.Rules = append(d.Rules, DiagnosticRule{"affine_degenerate", "有效点集近共线，仿射矩阵可能退化", ids, true})
	}
	d.Ready = true
	for _, rule := range d.Rules {
		if rule.Blocking {
			d.Ready = false
		}
	}
	return d
}

func quadrant(job domain.RegistrationJob, p domain.ControlPoint) int {
	q := 0
	if p.PixelX >= float64(job.ImageWidth)/2 {
		q++
	}
	if p.PixelY >= float64(job.ImageHeight)/2 {
		q += 2
	}
	return q
}

func clusteredIDs(job domain.RegistrationJob, points []domain.ControlPoint) []string {
	limit := math.Hypot(float64(job.ImageWidth), float64(job.ImageHeight)) * 0.02
	set := map[string]bool{}
	for i := 0; i < len(points); i++ {
		for k := i + 1; k < len(points); k++ {
			if math.Hypot(points[i].PixelX-points[k].PixelX, points[i].PixelY-points[k].PixelY) < limit {
				set[points[i].PointID], set[points[k].PointID] = true, true
			}
		}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func degenerate(job domain.RegistrationJob, points []domain.ControlPoint) bool {
	scale := float64(job.ImageWidth * job.ImageHeight)
	if scale <= 0 {
		return true
	}
	maxArea := 0.0
	for i := 0; i < len(points); i++ {
		for j := i + 1; j < len(points); j++ {
			for k := j + 1; k < len(points); k++ {
				a := math.Abs((points[j].PixelX-points[i].PixelX)*(points[k].PixelY-points[i].PixelY) - (points[k].PixelX-points[i].PixelX)*(points[j].PixelY-points[i].PixelY))
				maxArea = math.Max(maxArea, a)
			}
		}
	}
	return maxArea/scale < 1e-6
}

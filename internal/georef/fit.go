package georef

import (
	"errors"
	"map-registration-gate/internal/domain"
	"math"
	"sort"
)

type Result struct {
	Coefficients [6]float64
	Residuals    map[string]float64
	RMSE, Max    float64
	Distribution bool
}

func Fit(job domain.RegistrationJob, points []domain.ControlPoint) (Result, error) {
	active := make([]domain.ControlPoint, 0)
	for _, p := range points {
		if p.Active {
			active = append(active, p)
		}
	}
	if len(active) < 4 || !domain.Distribution(job, active) {
		return Result{}, errors.New("control points insufficient or distribution invalid")
	}
	sort.Slice(active, func(i, j int) bool { return active[i].PointID < active[j].PointID })
	var ata [6][6]float64
	var atb [6]float64
	for _, p := range active {
		rows := [2][6]float64{{p.PixelX, p.PixelY, 1, 0, 0, 0}, {0, 0, 0, p.PixelX, p.PixelY, 1}}
		bs := [2]float64{p.MapX, p.MapY}
		for r := 0; r < 2; r++ {
			for i := 0; i < 6; i++ {
				atb[i] += rows[r][i] * bs[r]
				for k := 0; k < 6; k++ {
					ata[i][k] += rows[r][i] * rows[r][k]
				}
			}
		}
	}
	x, ok := solve(ata, atb)
	if !ok {
		return Result{}, errors.New("degenerate control points")
	}
	var c [6]float64
	copy(c[:], x[:])
	res := map[string]float64{}
	sum := 0.0
	max := 0.0
	for _, p := range active {
		d := Residual(c, p.PixelX, p.PixelY, p.MapX, p.MapY)
		res[p.PointID] = d
		sum += d * d
		if d > max {
			max = d
		}
	}
	return Result{c, res, math.Sqrt(sum / float64(len(active))), max, true}, nil
}
func solve(a [6][6]float64, b [6]float64) ([6]float64, bool) {
	for i := 0; i < 6; i++ {
		p := i
		for r := i + 1; r < 6; r++ {
			if math.Abs(a[r][i]) > math.Abs(a[p][i]) {
				p = r
			}
		}
		if math.Abs(a[p][i]) < 1e-12 {
			return [6]float64{}, false
		}
		a[i], a[p] = a[p], a[i]
		b[i], b[p] = b[p], b[i]
		for r := i + 1; r < 6; r++ {
			f := a[r][i] / a[i][i]
			for k := i; k < 6; k++ {
				a[r][k] -= f * a[i][k]
			}
			b[r] -= f * b[i]
		}
	}
	var x [6]float64
	for i := 5; i >= 0; i-- {
		s := b[i]
		for k := i + 1; k < 6; k++ {
			s -= a[i][k] * x[k]
		}
		x[i] = s / a[i][i]
	}
	return x, true
}

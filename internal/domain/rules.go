package domain

import (
	"math"
	"strings"
)

func ValidatePoint(j RegistrationJob, points []ControlPoint, p ControlPoint) error {
	if j.State == Draft || j.State == Published {
		return ErrInvalidState
	}
	if strings.TrimSpace(p.EvidenceNote) == "" || strings.TrimSpace(p.CreatedBy) == "" {
		return ErrRule
	}
	if p.PixelX < 0 || p.PixelY < 0 || p.PixelX > float64(j.ImageWidth) || p.PixelY > float64(j.ImageHeight) || !finite(p.PixelX, p.PixelY, p.MapX, p.MapY) {
		return ErrRule
	}
	for _, x := range points {
		if x.Active && x.PointID != p.PointID && x.PixelX == p.PixelX && x.PixelY == p.PixelY {
			return ErrRule
		}
	}
	return nil
}
func finite(v ...float64) bool {
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return true
}
func Distribution(j RegistrationJob, points []ControlPoint) bool {
	q := [4]bool{}
	n := 0
	for _, p := range points {
		if !p.Active {
			continue
		}
		n++
		idx := 0
		if p.PixelX >= float64(j.ImageWidth)/2 {
			idx++
		}
		if p.PixelY >= float64(j.ImageHeight)/2 {
			idx += 2
		}
		q[idx] = true
	}
	return n >= 4 && q[0] && q[1] && q[2] && q[3]
}

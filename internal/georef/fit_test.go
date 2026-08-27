package georef

import (
	"map-registration-gate/internal/domain"
	"math"
	"reflect"
	"testing"
)

func fixture() (domain.RegistrationJob, []domain.ControlPoint) {
	j := domain.RegistrationJob{ImageWidth: 1000, ImageHeight: 800}
	coords := [][2]float64{{100, 100}, {900, 100}, {100, 700}, {900, 700}, {500, 300}}
	pts := make([]domain.ControlPoint, 0, len(coords))
	for i, p := range coords {
		x, y := Apply([6]float64{2, .25, 10, -.1, 3, 20}, p[0], p[1])
		pts = append(pts, domain.ControlPoint{PointID: string(rune('a' + i)), PixelX: p[0], PixelY: p[1], MapX: x, MapY: y, Active: true})
	}
	return j, pts
}

func TestDiagnoseNearCollinearPointSet(t *testing.T) {
	j := domain.RegistrationJob{ImageWidth: 1000, ImageHeight: 1000, Revision: 7}
	points := []domain.ControlPoint{{PointID: "a", PixelX: 100, PixelY: 499.999, Active: true}, {PointID: "b", PixelX: 900, PixelY: 500.001, Active: true}, {PointID: "c", PixelX: 200, PixelY: 499.9995, Active: true}, {PointID: "d", PixelX: 800, PixelY: 500.0005, Active: true}}
	d := Diagnose(j, points)
	found := false
	for _, r := range d.Rules {
		if r.Code == "affine_degenerate" && r.Blocking {
			found = true
		}
	}
	if !found || d.Ready {
		t.Fatalf("near-collinear set accepted: %+v", d)
	}
}
func TestFitDeterministicAffine(t *testing.T) {
	j, pts := fixture()
	a, e := Fit(j, pts)
	if e != nil {
		t.Fatal(e)
	}
	pts[0], pts[4] = pts[4], pts[0]
	b, e := Fit(j, pts)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(a.Coefficients, b.Coefficients) {
		t.Fatalf("order changed coefficients: %v %v", a.Coefficients, b.Coefficients)
	}
	if a.RMSE > 1e-8 || math.Abs(a.Coefficients[0]-2) > 1e-8 {
		t.Fatalf("unexpected fit: %+v", a)
	}
}
func TestFitRejectsDegenerate(t *testing.T) {
	j := domain.RegistrationJob{ImageWidth: 10, ImageHeight: 10}
	pts := []domain.ControlPoint{{PointID: "1", PixelX: 0, PixelY: 0, Active: true}, {PointID: "2", PixelX: 10, PixelY: 0, Active: true}, {PointID: "3", PixelX: 0, PixelY: 10, Active: true}}
	if _, e := Fit(j, pts); e == nil {
		t.Fatal("insufficient set accepted")
	}
}
func TestSampleStableAcrossOrder(t *testing.T) {
	_, pts := fixture()
	a := Sample(pts, "seed")
	pts[0], pts[3] = pts[3], pts[0]
	b := Sample(pts, "seed")
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("sample changed with input order: %v %v", a, b)
	}
	if len(a) < 3 {
		t.Fatalf("expected cross-region sample: %v", a)
	}
}

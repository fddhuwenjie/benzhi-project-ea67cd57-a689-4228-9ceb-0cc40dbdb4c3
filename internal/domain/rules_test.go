package domain

import (
	"errors"
	"testing"
)

func TestStateTransitions(t *testing.T) {
	j := RegistrationJob{State: Draft, Revision: 1}
	if e := j.Freeze(); e != nil {
		t.Fatal(e)
	}
	if j.State != Frozen || j.Revision != 2 {
		t.Fatalf("unexpected job: %+v", j)
	}
	if e := j.Publish(); !errors.Is(e, ErrInvalidState) {
		t.Fatalf("expected invalid state, got %v", e)
	}
	j.State = Published
	if e := j.SetSolvable(true); !errors.Is(e, ErrImmutable) {
		t.Fatalf("expected immutable, got %v", e)
	}
}
func TestDistributionAndPointBounds(t *testing.T) {
	j := RegistrationJob{ImageWidth: 100, ImageHeight: 80, State: Frozen}
	pts := []ControlPoint{{PixelX: 10, PixelY: 10, Active: true}, {PixelX: 90, PixelY: 10, Active: true}, {PixelX: 10, PixelY: 70, Active: true}, {PixelX: 90, PixelY: 70, Active: true}}
	if !Distribution(j, pts) {
		t.Fatal("expected four quadrant coverage")
	}
	p := ControlPoint{PixelX: 101, PixelY: 2, MapX: 1, MapY: 1, EvidenceNote: "证据", CreatedBy: "op"}
	if !errors.Is(ValidatePoint(j, pts, p), ErrRule) {
		t.Fatal("out of bounds point accepted")
	}
}
func TestBaselineDigestValidation(t *testing.T) {
	j := RegistrationJob{Title: "图", MapYear: 1930, ImageWidth: 10, ImageHeight: 10, ImageSHA256: "xyz", TargetCRS: "EPSG:3857", RMSELimit: 1, OperatorID: "op"}
	if !errors.Is(ValidateBaseline(j), ErrRule) {
		t.Fatal("malformed digest accepted")
	}
	j.ImageSHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if e := ValidateBaseline(j); e != nil {
		t.Fatal(e)
	}
}

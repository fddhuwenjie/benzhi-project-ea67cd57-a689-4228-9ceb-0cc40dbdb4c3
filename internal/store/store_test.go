package store

import (
	"errors"
	"map-registration-gate/internal/domain"
	"path/filepath"
	"testing"
)

func TestUpdateRollbackAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	sentinel := errors.New("stop")
	e = s.Update(func(d *Data) error {
		d.Jobs["bad"] = validJob("bad")
		return sentinel
	})
	if !errors.Is(e, sentinel) {
		t.Fatal(e)
	}
	if _, ok := s.Snapshot().Jobs["bad"]; ok {
		t.Fatal("failed transaction leaked mutation")
	}
	if e = s.Update(func(d *Data) error {
		d.Jobs["ok"] = validJob("ok")
		AddEvent(d, "ok", "created", "")
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	reloaded, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	if reloaded.Snapshot().Jobs["ok"].State != domain.Draft {
		t.Fatal("persisted state missing")
	}
}
func validJob(id string) domain.RegistrationJob {
	return domain.RegistrationJob{JobID: id, Title: "历史图", MapYear: 1930, ImageWidth: 10, ImageHeight: 10, ImageSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", TargetCRS: "EPSG:3857", RMSELimit: 1, State: domain.Draft, Revision: 1, OperatorID: "op"}
}
func TestAuditValidationDetectsTamper(t *testing.T) {
	d := Data{Jobs: map[string]domain.RegistrationJob{}, Manifests: map[string]domain.ReleaseManifest{}}
	AddEvent(&d, "j", "one", "")
	d.Events[0].Hash = "tampered"
	if validate(d) == nil {
		t.Fatal("tampered audit accepted")
	}
}

package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"map-registration-gate/internal/domain"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Data struct {
	Jobs               map[string]domain.RegistrationJob
	Points             map[string][]domain.ControlPoint
	Evals              map[string]domain.FitEvaluation
	EvalHistory        map[string][]domain.FitEvaluation
	Reviews            map[string]domain.Review
	Remediations       map[string][]domain.Remediation
	RemediationItems   map[string][]domain.RemediationItem
	ReturnInstructions map[string][]domain.ReturnInstruction
	Manifests          map[string]domain.ReleaseManifest
	Requests           map[string]Request
	Events             []domain.AuditEvent
}
type Request struct {
	Hash     string
	Response any
}
type Store struct {
	mu   sync.RWMutex
	path string
	d    Data
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, d: Data{Evals: map[string]domain.FitEvaluation{}, EvalHistory: map[string][]domain.FitEvaluation{}, Reviews: map[string]domain.Review{}, Remediations: map[string][]domain.Remediation{}, RemediationItems: map[string][]domain.RemediationItem{}, ReturnInstructions: map[string][]domain.ReturnInstruction{}, Requests: map[string]Request{}}}
	loaded := false
	if b, e := os.ReadFile(path); e == nil {
		loaded = true
		if e = json.Unmarshal(b, &s.d); e != nil {
			return nil, e
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if !loaded {
		s.d.Jobs = map[string]domain.RegistrationJob{}
		s.d.Points = map[string][]domain.ControlPoint{}
		s.d.Manifests = map[string]domain.ReleaseManifest{}
	}
	if s.d.Remediations == nil {
		s.d.Remediations = map[string][]domain.Remediation{}
	}
	if s.d.EvalHistory == nil {
		s.d.EvalHistory = map[string][]domain.FitEvaluation{}
	}
	if s.d.RemediationItems == nil {
		s.d.RemediationItems = map[string][]domain.RemediationItem{}
	}
	if s.d.ReturnInstructions == nil {
		s.d.ReturnInstructions = map[string][]domain.ReturnInstruction{}
	}
	if s.d.Evals == nil {
		s.d.Evals = map[string]domain.FitEvaluation{}
	}
	if s.d.Reviews == nil {
		s.d.Reviews = map[string]domain.Review{}
	}
	if s.d.Requests == nil {
		s.d.Requests = map[string]Request{}
	}
	for id, evaluation := range s.d.Evals {
		if len(s.d.EvalHistory[id]) == 0 {
			s.d.EvalHistory[id] = []domain.FitEvaluation{evaluation}
		}
		job := s.d.Jobs[id]
		if job.CurrentEvaluationID == "" && job.State != domain.Draft && job.State != domain.Frozen {
			job.CurrentEvaluationID = evaluation.EvaluationID
			s.d.Jobs[id] = job
		}
	}
	if e := validate(s.d); e != nil {
		return nil, e
	}
	return s, nil
}
func (s *Store) persist(d Data) error {
	b, e := json.Marshal(d)
	if e != nil {
		return e
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if e = os.WriteFile(tmp, b, 0644); e != nil {
		return e
	}
	return os.Rename(tmp, s.path)
}
func clone(d Data) (Data, error) {
	b, e := json.Marshal(d)
	if e != nil {
		return Data{}, e
	}
	var out Data
	e = json.Unmarshal(b, &out)
	return out, e
}
func (s *Store) Snapshot() Data { s.mu.RLock(); defer s.mu.RUnlock(); out, _ := clone(s.d); return out }
func (s *Store) Update(fn func(*Data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next, e := clone(s.d)
	if e != nil {
		return e
	}
	if e = fn(&next); e != nil {
		return e
	}
	if e = validate(next); e != nil {
		return e
	}
	if e = s.persist(next); e != nil {
		return e
	}
	s.d = next
	return nil
}
func Hash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func AddEvent(d *Data, job, typ, detail string) {
	prev := ""
	before := domain.State("")
	if len(d.Events) > 0 {
		prev = d.Events[len(d.Events)-1].Hash
	}
	for i := len(d.Events) - 1; i >= 0; i-- {
		if d.Events[i].JobID == job {
			before = d.Events[i].AfterState
			break
		}
	}
	j := d.Jobs[job]
	e := domain.AuditEvent{Seq: uint64(len(d.Events) + 1), JobID: job, Type: typ, Detail: detail, BeforeState: before, AfterState: j.State, Revision: j.Revision, PrevHash: prev, At: time.Now().UTC()}
	e.Hash = eventHash(e)
	d.Events = append(d.Events, e)
}
func eventHash(e domain.AuditEvent) string {
	return Hash(fmt.Sprintf("%d|%s|%s|%s|%s|%s|%d|%s|%s", e.Seq, e.JobID, e.Type, e.Detail, e.BeforeState, e.AfterState, e.Revision, e.PrevHash, e.At.Format(time.RFC3339Nano)))
}
func validate(d Data) error {
	prev := ""
	for i, e := range d.Events {
		if e.Seq != uint64(i+1) || e.PrevHash != prev {
			return fmt.Errorf("audit chain discontinuity at %d", i+1)
		}
		want := eventHash(e)
		if e.Hash != want {
			return fmt.Errorf("audit hash invalid at %d", i+1)
		}
		prev = e.Hash
	}
	for id, j := range d.Jobs {
		if !j.State.Known() {
			return fmt.Errorf("unknown state for job %s", id)
		}
		var evaluation *domain.FitEvaluation
		if value, ok := d.Evals[id]; ok {
			copy := value
			evaluation = &copy
		}
		var review *domain.Review
		if value, ok := d.Reviews[id]; ok {
			copy := value
			review = &copy
		}
		var manifest *domain.ReleaseManifest
		if value, ok := d.Manifests[id]; ok {
			copy := value
			manifest = &copy
		}
		evidence := domain.AggregateEvidence{Job: j, Points: d.Points[id], Evaluation: evaluation, Review: review, Manifest: manifest, Remediations: d.Remediations[id]}
		if err := domain.ValidateAggregate(evidence); err != nil {
			return fmt.Errorf("job %s integrity: %w", id, err)
		}
		if j.CurrentEvaluationID != "" {
			evaluation, ok := d.Evals[id]
			if !ok || evaluation.EvaluationID != j.CurrentEvaluationID {
				return fmt.Errorf("job %s current evaluation reference invalid", id)
			}
			found := false
			for _, historical := range d.EvalHistory[id] {
				if historical.EvaluationID == j.CurrentEvaluationID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("job %s current evaluation missing from history", id)
			}
		}
	}
	for id, m := range d.Manifests {
		j, ok := d.Jobs[id]
		if !ok || j.State != domain.Published || m.JobID != id {
			return fmt.Errorf("invalid published manifest %s", id)
		}
	}
	return nil
}

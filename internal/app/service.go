package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/store"
	"sync"
	"time"
)

type Service struct {
	store *store.Store
	locks sync.Map
	now   func() time.Time
}

func New(s *store.Store) *Service {
	return &Service{store: s, now: func() time.Time { return time.Now().UTC() }}
}
func (s *Service) lock(id string) func() {
	v, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}
func stableID(prefix, requestID string) string {
	digest := requestHash(struct{ RequestID string }{requestID})
	return prefix + "-" + digest[:16]
}
func validRequest(id string) error {
	if len(id) < 4 || len(id) > 128 {
		return fmt.Errorf("%w: request_id", domain.ErrRule)
	}
	return nil
}
func checkRevision(j domain.RegistrationJob, want uint64) error {
	if j.Revision != want {
		return domain.ErrConflict
	}
	if j.State == domain.Published {
		return domain.ErrImmutable
	}
	return nil
}
func requestHash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func replay(d *store.Data, id string, cmd any) (Result, bool, error) {
	r, ok := d.Requests[id]
	if !ok {
		return Result{}, false, nil
	}
	if r.Hash != requestHash(cmd) {
		return Result{}, false, domain.ErrIdempotency
	}
	b, _ := json.Marshal(r.Response)
	var out Result
	_ = json.Unmarshal(b, &out)
	out.Replayed = true
	return out, true, nil
}
func remember(d *store.Data, id string, cmd any, r Result) {
	d.Requests[id] = store.Request{Hash: requestHash(cmd), Response: r}
}

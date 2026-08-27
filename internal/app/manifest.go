package app

import (
	"encoding/json"
	"fmt"
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/store"
	"time"
)

type canonicalManifest struct {
	JobID, BaselineDigest, ControlSetDigest, EvaluationDigest, ReviewDigest, ReviewerID, ReviewDecision, AuditHead string
}

func canonical(j domain.RegistrationJob, p []domain.ControlPoint, e domain.FitEvaluation, r domain.Review, audit string) canonicalManifest {
	return canonicalManifest{j.JobID, domain.BaselineDigest(j), domain.ControlSetDigest(p), domain.Digest(e), domain.ReviewDigest(r), r.ReviewerID, r.Decision, audit}
}
func canonicalDigest(c canonicalManifest) string {
	raw, _ := json.Marshal(c)
	return domain.Digest(json.RawMessage(raw))
}

func (s *Service) Release(c RevisionCommand) (Result, error) {
	if err := validRequest(c.RequestID); err != nil {
		return Result{}, err
	}
	unlock := s.lock(c.JobID)
	defer unlock()
	var out Result
	err := s.store.Update(func(d *store.Data) error {
		if r, ok, err := replay(d, c.RequestID, c); ok || err != nil {
			out = r
			return err
		}
		j, ok := d.Jobs[c.JobID]
		if !ok {
			return domain.ErrNotFound
		}
		if err := checkRevision(j, c.ExpectedRevision); err != nil {
			return err
		}
		if _, ok := d.Manifests[j.JobID]; ok {
			return domain.ErrImmutable
		}
		review, ok := d.Reviews[j.JobID]
		if !ok || review.Decision != "approve" || review.SampleDigest == "" || review.SampleDigest != j.ReviewSampleDigest {
			return domain.ErrInvalidState
		}
		evaluation, ok := d.Evals[j.JobID]
		if !ok || evaluation.Decision != "pass" || j.CurrentEvaluationID != evaluation.EvaluationID || evaluation.PointSetDigest != domain.ControlSetDigest(d.Points[j.JobID]) {
			return fmt.Errorf("%w: current passing evaluation required", domain.ErrRule)
		}
		cm := canonical(j, d.Points[j.JobID], evaluation, review, store.AuditHead(*d, j.JobID))
		m := domain.ReleaseManifest{ManifestID: stableID("manifest", c.RequestID), JobID: j.JobID, BaselineDigest: cm.BaselineDigest, ControlSetDigest: cm.ControlSetDigest, EvaluationDigest: cm.EvaluationDigest, ReviewDigest: cm.ReviewDigest, ReviewerID: cm.ReviewerID, ReviewDecision: cm.ReviewDecision, AuditHead: cm.AuditHead, CanonicalSHA256: canonicalDigest(cm), ReleasedAt: s.now()}
		if err := j.Publish(); err != nil {
			return err
		}
		d.Manifests[j.JobID] = m
		d.Jobs[j.JobID] = j
		store.AddEvent(d, j.JobID, "manifest.published", m.CanonicalSHA256)
		out = resultFor(j)
		remember(d, c.RequestID, c, out)
		return nil
	})
	return out, err
}

type ManifestComponentReport struct {
	ExpectedDigest     string `json:"expected_digest,omitempty"`
	RecalculatedDigest string `json:"recalculated_digest,omitempty"`
	Passed             bool   `json:"passed"`
	Reason             string `json:"reason,omitempty"`
}
type ManifestComponents struct {
	Baseline   ManifestComponentReport `json:"baseline"`
	ControlSet ManifestComponentReport `json:"control_set"`
	Evaluation ManifestComponentReport `json:"evaluation"`
	Review     ManifestComponentReport `json:"review"`
	AuditChain ManifestComponentReport `json:"audit_chain"`
	Total      ManifestComponentReport `json:"total"`
}
type ManifestVerificationReport struct {
	Valid              bool               `json:"valid"`
	CheckedAt          time.Time          `json:"checked_at"`
	Components         ManifestComponents `json:"components"`
	RecalculatedSHA256 string             `json:"recalculated_sha256"`
}

func component(expected, actual, reason string) ManifestComponentReport {
	passed := expected != "" && expected == actual && reason == ""
	if reason == "" && !passed {
		reason = "digest_mismatch"
	}
	return ManifestComponentReport{expected, actual, passed, reason}
}
func (s *Service) VerifyManifestReport(jobID string) (ManifestVerificationReport, error) {
	d := s.store.Snapshot()
	j, ok := d.Jobs[jobID]
	if !ok {
		return ManifestVerificationReport{}, domain.ErrNotFound
	}
	m, ok := d.Manifests[jobID]
	if !ok {
		return ManifestVerificationReport{}, fmt.Errorf("%w: release manifest evidence missing", domain.ErrNotFound)
	}
	report := ManifestVerificationReport{CheckedAt: s.now()}
	report.Components.Baseline = component(m.BaselineDigest, domain.BaselineDigest(j), "")
	report.Components.ControlSet = component(m.ControlSetDigest, domain.ControlSetDigest(d.Points[jobID]), "")
	evaluation, evalOK := d.Evals[jobID]
	evalDigest, evalReason := "", ""
	if !evalOK || evaluation.EvaluationID == "" {
		evalReason = "evaluation_evidence_missing"
	} else {
		evalDigest = domain.Digest(evaluation)
	}
	report.Components.Evaluation = component(m.EvaluationDigest, evalDigest, evalReason)
	review, reviewOK := d.Reviews[jobID]
	reviewDigest, reviewReason := "", ""
	if !reviewOK || review.ReviewerID == "" {
		reviewReason = "review_evidence_missing"
	} else {
		reviewDigest = domain.ReviewDigest(review)
	}
	report.Components.Review = component(m.ReviewDigest, reviewDigest, reviewReason)
	audit := store.VerifyAuditEvidence(d, jobID, m.AuditHead)
	report.Components.AuditChain = component(m.AuditHead, audit.RecalculatedHead, audit.Reason)
	cm := canonicalManifest{jobID, report.Components.Baseline.RecalculatedDigest, report.Components.ControlSet.RecalculatedDigest, report.Components.Evaluation.RecalculatedDigest, report.Components.Review.RecalculatedDigest, m.ReviewerID, m.ReviewDecision, report.Components.AuditChain.RecalculatedDigest}
	report.RecalculatedSHA256 = canonicalDigest(cm)
	report.Components.Total = component(m.CanonicalSHA256, report.RecalculatedSHA256, "")
	report.Valid = report.Components.Baseline.Passed && report.Components.ControlSet.Passed && report.Components.Evaluation.Passed && report.Components.Review.Passed && report.Components.AuditChain.Passed && report.Components.Total.Passed
	return report, nil
}
func (s *Service) VerifyManifest(jobID string) (bool, string, error) {
	r, e := s.VerifyManifestReport(jobID)
	return r.Valid, r.RecalculatedSHA256, e
}

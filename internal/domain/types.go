package domain

import (
	"errors"
	"time"
)

type State string

const (
	Draft          State = "draft"
	Frozen         State = "frozen"
	Solvable       State = "solvable"
	NeedsFix       State = "needs_fix"
	PendingReview  State = "pending_review"
	PendingRelease State = "pending_release"
	Published      State = "published"
)

type RegistrationJob struct {
	JobID, Title                     string
	MapYear, ImageWidth, ImageHeight int
	ImageSHA256, TargetCRS           string
	RMSELimit                        float64
	State                            State
	Revision                         uint64
	OperatorID                       string
	CurrentEvaluationID              string
	ReviewSampleDigest               string
	CreatedAt                        time.Time
}
type ControlPoint struct {
	PointID, JobID               string
	PixelX, PixelY, MapX, MapY   float64
	EvidenceNote                 string
	Active                       bool
	SupersedesPointID, CreatedBy string
	CreatedAt                    time.Time
}
type FitEvaluation struct {
	EvaluationID, JobID string
	InputRevision       uint64
	PointSetDigest      string
	Coefficients        [6]float64
	PointResiduals      map[string]float64
	RMSE, MaxResidual   float64
	DistributionPassed  bool
	Decision            string
	RuleFailures        []string
	OutlierPointIDs     []string
	EvaluatedAt         time.Time
}
type EvaluationComparison struct {
	FromEvaluationID, ToEvaluationID string
	RMSEChange, MaxResidualChange    float64
	OutlierCountChange               int
	CoefficientChanges               [6]float64
	Trend                            string
}
type ReviewItem struct {
	PointID   string `json:"point_id"`
	IssueType string `json:"issue_type"`
	Note      string `json:"note"`
	Passed    bool   `json:"passed"`
}
type Review struct {
	ReviewerID   string
	Samples      map[string]bool
	SampleDigest string
	Items        []ReviewItem
	Notes        string
	Decision     string
	CreatedAt    time.Time
}
type ReturnInstruction struct {
	InstructionID, JobID, PointID, IssueType, Note string
	ReviewSampleDigest                             string
	Status                                         string
	CreatedAt                                      time.Time
}
type RemediationItem struct {
	ItemID, JobID, PointID, EvaluationID, RuleSource string
	TriggerResidual, LatestResidual                  float64
	Status, ReplacementPointID                       string
	CreatedAt, UpdatedAt                             time.Time
}
type Remediation struct {
	RemediationID, JobID, ItemID, OldPointID, NewPointID string
	Reason, ReplacementEvidence, ActorID                 string
	CreatedAt                                            time.Time
}
type ReleaseManifest struct {
	ManifestID, JobID, BaselineDigest, ControlSetDigest, EvaluationDigest, ReviewDigest, ReviewerID, ReviewDecision, AuditHead, CanonicalSHA256 string
	ReleasedAt                                                                                                                                  time.Time
}
type AuditEvent struct {
	Seq                     uint64
	JobID, Type, Detail     string
	BeforeState, AfterState State
	Revision                uint64
	PrevHash, Hash          string
	At                      time.Time
}

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("revision conflict")
var ErrInvalidState = errors.New("invalid state")
var ErrRule = errors.New("rule violation")
var ErrImmutable = errors.New("immutable")
var ErrIdempotency = errors.New("idempotency conflict")

func (j *RegistrationJob) Freeze() error {
	if j.State != Draft {
		return ErrInvalidState
	}
	j.State = Frozen
	j.Revision++
	return nil
}
func (j *RegistrationJob) SetSolvable(ok bool) error {
	if j.State == Published {
		return ErrImmutable
	}
	if ok {
		j.State = Solvable
	} else {
		j.State = NeedsFix
	}
	j.Revision++
	return nil
}
func (j *RegistrationJob) SubmitReview() error {
	if j.State != Solvable {
		return ErrInvalidState
	}
	j.State = PendingReview
	j.Revision++
	return nil
}
func (j *RegistrationJob) AcceptReview() error {
	if j.State != PendingReview {
		return ErrInvalidState
	}
	j.State = PendingRelease
	j.Revision++
	return nil
}
func (j *RegistrationJob) RejectReview() error {
	if j.State != PendingReview {
		return ErrInvalidState
	}
	j.State = NeedsFix
	j.Revision++
	return nil
}
func (j *RegistrationJob) Publish() error {
	if j.State != PendingRelease {
		return ErrInvalidState
	}
	j.State = Published
	j.Revision++
	return nil
}

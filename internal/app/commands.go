package app

import "map-registration-gate/internal/domain"

type CreateJobCommand struct {
	RequestID, JobID, Title, ImageSHA256, TargetCRS, OperatorID string
	MapYear, ImageWidth, ImageHeight                            int
	RMSELimit                                                   float64
}
type RevisionCommand struct {
	RequestID, JobID string
	ExpectedRevision uint64
}
type ReviseBaselineCommand struct {
	RequestID, JobID, Title, ImageSHA256, TargetCRS string
	ExpectedRevision                                uint64
	MapYear, ImageWidth, ImageHeight                int
	RMSELimit                                       float64
}
type PointCommand struct {
	RequestID, JobID, PointID  string
	ExpectedRevision           uint64
	PixelX, PixelY, MapX, MapY float64
	EvidenceNote, ActorID      string
}
type ReplacePointCommand struct {
	RequestID, JobID, OldPointID, NewPointID string
	ExpectedRevision                         uint64
	PixelX, PixelY, MapX, MapY               float64
	Reason, EvidenceNote, ActorID            string
	RemediationItemID, PreviewDigest         string
}
type BatchPoint struct {
	PointID      string  `json:"point_id"`
	PixelX       float64 `json:"pixel_x"`
	PixelY       float64 `json:"pixel_y"`
	MapX         float64 `json:"map_x"`
	MapY         float64 `json:"map_y"`
	EvidenceNote string  `json:"evidence_note"`
	ActorID      string  `json:"actor_id"`
}
type BatchPointsCommand struct {
	RequestID, JobID, PreviewDigest string
	ExpectedRevision                uint64
	Points                          []BatchPoint
}
type RowError struct {
	Row     int    `json:"row"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type BatchPreview struct {
	PreviewDigest    string     `json:"preview_digest"`
	Errors           []RowError `json:"errors"`
	ActiveAfter      int        `json:"active_after"`
	QuadrantCounts   [4]int     `json:"quadrant_counts"`
	MissingQuadrants []int      `json:"missing_quadrants"`
	Solvable         bool       `json:"solvable"`
}
type ReplacementPreview struct {
	PreviewDigest     string             `json:"preview_digest"`
	BeforeRMSE        float64            `json:"before_rmse"`
	AfterRMSE         float64            `json:"after_rmse"`
	BeforeMaxResidual float64            `json:"before_max_residual"`
	AfterMaxResidual  float64            `json:"after_max_residual"`
	ResidualChanges   map[string]float64 `json:"residual_changes"`
	BeforeQuadrants   [4]int             `json:"before_quadrants"`
	AfterQuadrants    [4]int             `json:"after_quadrants"`
	RuleFailures      []string           `json:"rule_failures"`
	WouldPass         bool               `json:"would_pass"`
}
type UpdatePointCommand struct {
	RequestID, JobID, PointID  string
	ExpectedRevision           uint64
	PixelX, PixelY, MapX, MapY float64
	EvidenceNote, ActorID      string
}
type DeactivatePointCommand struct {
	RequestID, JobID, PointID string
	ExpectedRevision          uint64
	Reason, ActorID           string
}
type ReviewCommand struct {
	RequestID, JobID, ReviewerID, Decision, Notes string
	ExpectedRevision                              uint64
	Samples                                       map[string]bool
	SampleDigest                                  string
	Items                                         []domain.ReviewItem
}
type Result struct {
	JobID    string `json:"job_id"`
	Revision uint64 `json:"revision"`
	State    string `json:"state"`
	Replayed bool   `json:"replayed,omitempty"`
}

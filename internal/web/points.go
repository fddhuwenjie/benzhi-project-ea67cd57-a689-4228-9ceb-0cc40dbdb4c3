package web

import (
	"net/http"

	"map-registration-gate/internal/app"
)

type addPointBody struct {
	RequestID        string  `json:"request_id"`
	PointID          string  `json:"point_id"`
	ExpectedRevision uint64  `json:"expected_revision"`
	PixelX           float64 `json:"pixel_x"`
	PixelY           float64 `json:"pixel_y"`
	MapX             float64 `json:"map_x"`
	MapY             float64 `json:"map_y"`
	EvidenceNote     string  `json:"evidence_note"`
	ActorID          string  `json:"actor_id"`
}

type remediationBody struct {
	RequestID         string  `json:"request_id"`
	OldPointID        string  `json:"old_point_id"`
	NewPointID        string  `json:"new_point_id"`
	ExpectedRevision  uint64  `json:"expected_revision"`
	PixelX            float64 `json:"pixel_x"`
	PixelY            float64 `json:"pixel_y"`
	MapX              float64 `json:"map_x"`
	MapY              float64 `json:"map_y"`
	Reason            string  `json:"reason"`
	EvidenceNote      string  `json:"evidence_note"`
	ActorID           string  `json:"actor_id"`
	RemediationItemID string  `json:"remediation_item_id"`
	PreviewDigest     string  `json:"preview_digest"`
}
type batchBody struct {
	RequestID        string           `json:"request_id"`
	ExpectedRevision uint64           `json:"expected_revision"`
	PreviewDigest    string           `json:"preview_digest"`
	Points           []app.BatchPoint `json:"points"`
}

type updatePointBody struct {
	RequestID        string  `json:"request_id"`
	ExpectedRevision uint64  `json:"expected_revision"`
	PixelX           float64 `json:"pixel_x"`
	PixelY           float64 `json:"pixel_y"`
	MapX             float64 `json:"map_x"`
	MapY             float64 `json:"map_y"`
	EvidenceNote     string  `json:"evidence_note"`
	ActorID          string  `json:"actor_id"`
}

type deactivatePointBody struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision uint64 `json:"expected_revision"`
	Reason           string `json:"reason"`
	ActorID          string `json:"actor_id"`
}

func (s *Server) AddPointHandler(w http.ResponseWriter, r *http.Request) {
	var body addPointBody
	if !decode(w, r, &body) {
		return
	}
	command := app.PointCommand{
		RequestID:        body.RequestID,
		JobID:            r.PathValue("job"),
		PointID:          body.PointID,
		ExpectedRevision: body.ExpectedRevision,
		PixelX:           body.PixelX,
		PixelY:           body.PixelY,
		MapX:             body.MapX,
		MapY:             body.MapY,
		EvidenceNote:     body.EvidenceNote,
		ActorID:          body.ActorID,
	}
	result, err := s.app.AddPoint(command)
	if err != nil {
		appError(w, err)
		return
	}
	respond(w, http.StatusCreated, result)
}

func batchCommand(r *http.Request, b batchBody) app.BatchPointsCommand {
	return app.BatchPointsCommand{RequestID: b.RequestID, JobID: r.PathValue("job"), ExpectedRevision: b.ExpectedRevision, PreviewDigest: b.PreviewDigest, Points: b.Points}
}
func (s *Server) BatchPreflightHandler(w http.ResponseWriter, r *http.Request) {
	var b batchBody
	if !decode(w, r, &b) {
		return
	}
	out, err := s.app.PreviewBatch(batchCommand(r, b))
	if err != nil {
		appError(w, err)
		return
	}
	respond(w, http.StatusOK, out)
}
func (s *Server) BatchImportHandler(w http.ResponseWriter, r *http.Request) {
	var b batchBody
	if !decode(w, r, &b) {
		return
	}
	out, err := s.app.ImportBatch(batchCommand(r, b))
	if err != nil {
		appError(w, err)
		return
	}
	respond(w, http.StatusCreated, out)
}
func (s *Server) RemediationPreviewHandler(w http.ResponseWriter, r *http.Request) {
	var b remediationBody
	if !decode(w, r, &b) {
		return
	}
	c := app.ReplacePointCommand{JobID: r.PathValue("job"), OldPointID: b.OldPointID, NewPointID: b.NewPointID, ExpectedRevision: b.ExpectedRevision, PixelX: b.PixelX, PixelY: b.PixelY, MapX: b.MapX, MapY: b.MapY, Reason: b.Reason, EvidenceNote: b.EvidenceNote, ActorID: b.ActorID, RemediationItemID: b.RemediationItemID}
	out, err := s.app.PreviewReplacement(c)
	if err != nil {
		appError(w, err)
		return
	}
	respond(w, http.StatusOK, out)
}

func (s *Server) RemediateHandler(w http.ResponseWriter, r *http.Request) {
	var body remediationBody
	if !decode(w, r, &body) {
		return
	}
	command := app.ReplacePointCommand{
		RequestID:         body.RequestID,
		JobID:             r.PathValue("job"),
		OldPointID:        body.OldPointID,
		NewPointID:        body.NewPointID,
		ExpectedRevision:  body.ExpectedRevision,
		PixelX:            body.PixelX,
		PixelY:            body.PixelY,
		MapX:              body.MapX,
		MapY:              body.MapY,
		Reason:            body.Reason,
		EvidenceNote:      body.EvidenceNote,
		ActorID:           body.ActorID,
		RemediationItemID: body.RemediationItemID,
		PreviewDigest:     body.PreviewDigest,
	}
	result, err := s.app.ReplacePoint(command)
	if err != nil {
		appError(w, err)
		return
	}
	respond(w, http.StatusCreated, result)
}

func (s *Server) UpdatePointHandler(w http.ResponseWriter, r *http.Request) {
	var body updatePointBody
	if !decode(w, r, &body) {
		return
	}
	command := app.UpdatePointCommand{
		RequestID:        body.RequestID,
		JobID:            r.PathValue("job"),
		PointID:          r.PathValue("point"),
		ExpectedRevision: body.ExpectedRevision,
		PixelX:           body.PixelX,
		PixelY:           body.PixelY,
		MapX:             body.MapX,
		MapY:             body.MapY,
		EvidenceNote:     body.EvidenceNote,
		ActorID:          body.ActorID,
	}
	result, err := s.app.UpdatePoint(command)
	if err != nil {
		appError(w, err)
		return
	}
	respond(w, http.StatusOK, result)
}

func (s *Server) DeactivatePointHandler(w http.ResponseWriter, r *http.Request) {
	var body deactivatePointBody
	if !decode(w, r, &body) {
		return
	}
	command := app.DeactivatePointCommand{
		RequestID:        body.RequestID,
		JobID:            r.PathValue("job"),
		PointID:          r.PathValue("point"),
		ExpectedRevision: body.ExpectedRevision,
		Reason:           body.Reason,
		ActorID:          body.ActorID,
	}
	result, err := s.app.DeactivatePoint(command)
	if err != nil {
		appError(w, err)
		return
	}
	respond(w, http.StatusOK, result)
}

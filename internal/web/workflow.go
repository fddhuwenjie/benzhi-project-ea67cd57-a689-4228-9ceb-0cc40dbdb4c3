package web

import (
	"net/http"

	"map-registration-gate/internal/app"
	"map-registration-gate/internal/domain"
)

type reviewBody struct {
	RequestID        string              `json:"request_id"`
	ExpectedRevision uint64              `json:"expected_revision"`
	ReviewerID       string              `json:"reviewer_id"`
	Decision         string              `json:"decision"`
	Notes            string              `json:"notes"`
	Samples          map[string]bool     `json:"samples"`
	SampleDigest     string              `json:"sample_digest"`
	Items            []domain.ReviewItem `json:"items"`
}

func revisionCommand(r *http.Request, body revisionBody) app.RevisionCommand {
	return app.RevisionCommand{
		RequestID:        body.RequestID,
		JobID:            r.PathValue("job"),
		ExpectedRevision: body.ExpectedRevision,
	}
}

func (s *Server) EvaluateHandler(w http.ResponseWriter, r *http.Request) {
	var body revisionBody
	if !decode(w, r, &body) {
		return
	}
	result, err := s.app.Evaluate(revisionCommand(r, body))
	if err != nil {
		appError(w, err)
		return
	}
	respond(w, http.StatusOK, result)
}

func (s *Server) ReviewHandler(w http.ResponseWriter, r *http.Request) {
	var body reviewBody
	if !decode(w, r, &body) {
		return
	}
	command := app.ReviewCommand{
		RequestID:        body.RequestID,
		JobID:            r.PathValue("job"),
		ReviewerID:       body.ReviewerID,
		Decision:         body.Decision,
		Notes:            body.Notes,
		ExpectedRevision: body.ExpectedRevision,
		Samples:          body.Samples,
		SampleDigest:     body.SampleDigest,
		Items:            body.Items,
	}
	result, err := s.app.Review(command)
	if err != nil {
		appError(w, err)
		return
	}
	respond(w, http.StatusOK, result)
}

func (s *Server) ReleaseHandler(w http.ResponseWriter, r *http.Request) {
	var body revisionBody
	if !decode(w, r, &body) {
		return
	}
	result, err := s.app.Release(revisionCommand(r, body))
	if err != nil {
		appError(w, err)
		return
	}
	respond(w, http.StatusCreated, result)
}

func (s *Server) VerifyManifestHandler(w http.ResponseWriter, r *http.Request) {
	report, err := s.app.VerifyManifestReport(r.PathValue("job"))
	if err != nil {
		appError(w, err)
		return
	}
	respond(w, http.StatusOK, report)
}

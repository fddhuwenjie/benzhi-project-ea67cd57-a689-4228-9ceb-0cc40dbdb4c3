package web

import (
	"map-registration-gate/internal/app"
	"net/http"
)

func (s *Server) WorkbenchHandler(w http.ResponseWriter, r *http.Request) {
	b, e := assets.ReadFile("assets/index.html")
	if e != nil {
		http.Error(w, "resource unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, map[string]string{"status": "ok"})
}
func (s *Server) ListJobsHandler(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, map[string]any{"jobs": s.app.List()})
}
func (s *Server) GetJobHandler(w http.ResponseWriter, r *http.Request) {
	v, e := s.app.Get(r.PathValue("job"))
	if e != nil {
		appError(w, e)
		return
	}
	respond(w, 200, v)
}
func (s *Server) CreateJobHandler(w http.ResponseWriter, r *http.Request) {
	var c app.CreateJobCommand
	if !decode(w, r, &c) {
		return
	}
	out, e := s.app.CreateJob(c)
	if e != nil {
		appError(w, e)
		return
	}
	respond(w, 201, out)
}
func (s *Server) FreezeHandler(w http.ResponseWriter, r *http.Request) {
	var b revisionBody
	if !decode(w, r, &b) {
		return
	}
	out, e := s.app.Freeze(app.RevisionCommand{RequestID: b.RequestID, JobID: r.PathValue("job"), ExpectedRevision: b.ExpectedRevision})
	if e != nil {
		appError(w, e)
		return
	}
	respond(w, 200, out)
}

type baselineBody struct {
	RequestID        string  `json:"request_id"`
	Title            string  `json:"title"`
	ImageSHA256      string  `json:"image_sha256"`
	TargetCRS        string  `json:"target_crs"`
	ExpectedRevision uint64  `json:"expected_revision"`
	MapYear          int     `json:"map_year"`
	ImageWidth       int     `json:"image_width"`
	ImageHeight      int     `json:"image_height"`
	RMSELimit        float64 `json:"rmse_limit"`
}

func (s *Server) BaselineConfirmationHandler(w http.ResponseWriter, r *http.Request) {
	out, err := s.app.BaselineConfirmation(r.PathValue("job"))
	if err != nil {
		appError(w, err)
		return
	}
	respond(w, http.StatusOK, out)
}
func (s *Server) ReviseBaselineHandler(w http.ResponseWriter, r *http.Request) {
	var b baselineBody
	if !decode(w, r, &b) {
		return
	}
	out, err := s.app.ReviseBaseline(app.ReviseBaselineCommand{RequestID: b.RequestID, JobID: r.PathValue("job"), Title: b.Title, ImageSHA256: b.ImageSHA256, TargetCRS: b.TargetCRS, ExpectedRevision: b.ExpectedRevision, MapYear: b.MapYear, ImageWidth: b.ImageWidth, ImageHeight: b.ImageHeight, RMSELimit: b.RMSELimit})
	if err != nil {
		appError(w, err)
		return
	}
	respond(w, http.StatusOK, out)
}
func (s *Server) SubmitReviewHandler(w http.ResponseWriter, r *http.Request) {
	var b revisionBody
	if !decode(w, r, &b) {
		return
	}
	out, e := s.app.SubmitReview(app.RevisionCommand{RequestID: b.RequestID, JobID: r.PathValue("job"), ExpectedRevision: b.ExpectedRevision})
	if e != nil {
		appError(w, e)
		return
	}
	respond(w, 200, out)
}

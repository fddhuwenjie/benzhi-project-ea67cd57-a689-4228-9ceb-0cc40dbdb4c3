package web

import (
	"embed"
	"io/fs"
	"map-registration-gate/internal/app"
	"net/http"
)

//go:embed assets/*
var assets embed.FS

type Server struct {
	app *app.Service
	mux *http.ServeMux
}

func New(a *app.Service) *Server        { s := &Server{app: a, mux: http.NewServeMux()}; s.routes(); return s }
func (s *Server) Handler() http.Handler { return security(s.mux) }
func (s *Server) routes() {
	sub, _ := fs.Sub(assets, "assets")
	s.mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(sub))))
	s.mux.HandleFunc("GET /", s.WorkbenchHandler)
	s.mux.HandleFunc("GET /api/jobs", s.ListJobsHandler)
	s.mux.HandleFunc("POST /api/jobs", s.CreateJobHandler)
	s.mux.HandleFunc("GET /api/jobs/{job}", s.GetJobHandler)
	s.mux.HandleFunc("POST /api/jobs/{job}/freeze", s.FreezeHandler)
	s.mux.HandleFunc("GET /api/jobs/{job}/baseline/confirmation", s.BaselineConfirmationHandler)
	s.mux.HandleFunc("PATCH /api/jobs/{job}/baseline", s.ReviseBaselineHandler)
	s.mux.HandleFunc("POST /api/jobs/{job}/points", s.AddPointHandler)
	s.mux.HandleFunc("POST /api/jobs/{job}/points/preflight", s.BatchPreflightHandler)
	s.mux.HandleFunc("POST /api/jobs/{job}/points/batch", s.BatchImportHandler)
	s.mux.HandleFunc("PATCH /api/jobs/{job}/points/{point}", s.UpdatePointHandler)
	s.mux.HandleFunc("DELETE /api/jobs/{job}/points/{point}", s.DeactivatePointHandler)
	s.mux.HandleFunc("POST /api/jobs/{job}/remediations", s.RemediateHandler)
	s.mux.HandleFunc("POST /api/jobs/{job}/remediations/preview", s.RemediationPreviewHandler)
	s.mux.HandleFunc("POST /api/jobs/{job}/evaluations", s.EvaluateHandler)
	s.mux.HandleFunc("POST /api/jobs/{job}/submit-review", s.SubmitReviewHandler)
	s.mux.HandleFunc("POST /api/jobs/{job}/reviews", s.ReviewHandler)
	s.mux.HandleFunc("POST /api/jobs/{job}/release", s.ReleaseHandler)
	s.mux.HandleFunc("GET /api/jobs/{job}/manifest/verify", s.VerifyManifestHandler)
	s.mux.HandleFunc("GET /healthz", s.HealthHandler)
}
func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

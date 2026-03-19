package api

import (
	"embed"
	"html/template"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/verinews/verinews/internal/pipeline"
	"github.com/verinews/verinews/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

type Server struct {
	pipeline *pipeline.Pipeline
	store    *store.Store
	tpl      *template.Template
}

func NewServer(p *pipeline.Pipeline, st *store.Store) (*Server, error) {
	funcMap := template.FuncMap{
		"nl2br": func(s string) template.HTML {
			escaped := template.HTMLEscapeString(s)
			return template.HTML(strings.ReplaceAll(escaped, "\n", "<br>"))
		},
		"scoreColor": func(score float64) string {
			switch {
			case score >= 81:
				return "#22c55e"
			case score >= 61:
				return "#3b82f6"
			case score >= 31:
				return "#f59e0b"
			default:
				return "#ef4444"
			}
		},
		"scoreLabel": func(score float64) string {
			switch {
			case score >= 81:
				return "Well-Supported"
			case score >= 61:
				return "Mostly Supported"
			case score >= 31:
				return "Disputed"
			default:
				return "Highly Contested"
			}
		},
	}
	tpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{pipeline: p, store: st, tpl: tpl}, nil
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/", s.handleIndex)
	r.Post("/analyze", s.handleAnalyze)
	r.Get("/articles/{id}/report", s.handleReport)

	return r
}

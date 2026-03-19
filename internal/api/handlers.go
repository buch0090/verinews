package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/verinews/verinews/internal/store"
	"github.com/verinews/verinews/internal/synthesis"
)

type indexData struct {
	Recent []store.ArticleRow
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	recent, err := s.store.ListArticles(r.Context(), 20)
	if err != nil {
		log.Printf("ListArticles: %v", err)
	}
	if err := s.tpl.ExecuteTemplate(w, "index.html", indexData{Recent: recent}); err != nil {
		log.Printf("index template: %v", err)
	}
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var rawURL string

	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		var req struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		rawURL = req.URL
	} else {
		rawURL = r.FormValue("url")
	}

	if rawURL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	id, err := s.store.CreatePending(r.Context(), rawURL)
	if err != nil {
		log.Printf("CreatePending %s: %v", rawURL, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	go func() {
		ctx := context.Background()
		if _, err := s.pipeline.Run(ctx, rawURL); err != nil {
			log.Printf("pipeline error for %s: %v", rawURL, err)
			if updateErr := s.store.UpdateStatus(ctx, id, "failed"); updateErr != nil {
				log.Printf("UpdateStatus failed: %v", updateErr)
			}
		}
	}()

	if strings.Contains(ct, "application/json") {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"article_id":%d}`, id)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/articles/%d/report", id), http.StatusSeeOther)
}

type reportData struct {
	Article    *store.ArticleRow
	Claims     []store.ClaimRow
	Analyses   []analysisDisplay
	Related    []store.RelatedArticle
	Label      string
	ScoreColor string
}

type analysisDisplay struct {
	Mode  string
	Title string
	Items []displayItem
}

type displayItem struct {
	Key   string
	Value string
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	article, err := s.store.GetArticle(r.Context(), id)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("GetArticle %d: %v", id, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := reportData{Article: article}

	if article.Status == "complete" {
		data.Label = synthesis.Label(article.TruthScore)
		data.ScoreColor = scoreColor(article.TruthScore)
		data.Claims, _ = s.store.GetClaims(r.Context(), id)
		analyses, _ := s.store.GetAnalyses(r.Context(), id)
		for _, a := range analyses {
			data.Analyses = append(data.Analyses, buildAnalysisDisplay(a))
		}
		data.Related, _ = s.store.GetRelatedArticlesByID(r.Context(), id)
	}

	if err := s.tpl.ExecuteTemplate(w, "report.html", data); err != nil {
		log.Printf("report template: %v", err)
	}
}

func scoreColor(score float64) string {
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
}

var philosopherTitles = map[string]string{
	"socratic":     "Socratic",
	"aristotelian": "Aristotelian",
	"platonic":     "Platonic",
	"stoic":        "Stoic",
}

func buildAnalysisDisplay(a store.AnalysisRow) analysisDisplay {
	title := philosopherTitles[a.Mode]
	if title == "" {
		title = strings.ToUpper(a.Mode[:1]) + a.Mode[1:]
	}
	disp := analysisDisplay{Mode: a.Mode, Title: title}

	var m map[string]any
	if err := json.Unmarshal(a.Output, &m); err != nil {
		return disp
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		disp.Items = append(disp.Items, displayItem{
			Key:   formatKey(k),
			Value: formatValue(m[k]),
		})
	}
	return disp
}

func formatKey(k string) string {
	words := strings.Split(k, "_")
	if len(words) == 0 {
		return k
	}
	words[0] = strings.ToUpper(words[0][:1]) + words[0][1:]
	return strings.Join(words, " ")
}

func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%.2f", val)
	case bool:
		if val {
			return "Yes"
		}
		return "No"
	case []any:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			switch s := item.(type) {
			case string:
				parts = append(parts, s)
			case map[string]any:
				b, _ := json.Marshal(s)
				parts = append(parts, string(b))
			default:
				parts = append(parts, fmt.Sprintf("%v", item))
			}
		}
		return strings.Join(parts, "\n")
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

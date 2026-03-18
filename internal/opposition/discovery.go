package opposition

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/verinews/verinews/internal/store"
)

type Discoverer struct {
	apiKey string
	client *http.Client
}

func New(apiKey string) *Discoverer {
	return &Discoverer{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

type newsAPIResponse struct {
	Status   string           `json:"status"`
	Message  string           `json:"message"`
	Articles []newsAPIArticle `json:"articles"`
}

type newsAPIArticle struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Source      struct {
		Name string `json:"name"`
	} `json:"source"`
}

// Discover searches NewsAPI for articles related to the given title.
// Returns up to 5 related articles.
func (d *Discoverer) Discover(ctx context.Context, title string) ([]store.RelatedArticle, error) {
	// Use first 8 words of title as query to keep it focused
	words := strings.Fields(title)
	if len(words) > 8 {
		words = words[:8]
	}
	query := strings.Join(words, " ")

	params := url.Values{}
	params.Set("q", query)
	params.Set("language", "en")
	params.Set("pageSize", "5")
	params.Set("sortBy", "relevancy")
	params.Set("apiKey", d.apiKey)

	reqURL := "https://newsapi.org/v2/everything?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("newsapi request: %w", err)
	}
	defer resp.Body.Close()

	var result newsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding newsapi response: %w", err)
	}
	if result.Status != "ok" {
		return nil, fmt.Errorf("newsapi error: %s", result.Message)
	}

	related := make([]store.RelatedArticle, 0, len(result.Articles))
	for _, a := range result.Articles {
		if a.URL == "" || a.Title == "[Removed]" {
			continue
		}
		related = append(related, store.RelatedArticle{
			URL:                     a.URL,
			Title:                   a.Title,
			Publisher:               a.Source.Name,
			Snippet:                 a.Description,
			RelationType:            "related",
			NarrativeDistanceScore:  0.5, // scored later in Phase 4
			ClaimContradictionScore: 0.0,
		})
	}
	return related, nil
}

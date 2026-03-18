package scraper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-readability"
)

const maxBodyBytes = 5 * 1024 * 1024 // 5MB cap

type Result struct {
	URL         string
	Title       string
	Author      string
	Publisher   string
	PublishedAt *time.Time
	RawHTML     string
	Content     string // cleaned plain text, ready for LLM
}

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *Scraper) Scrape(ctx context.Context, rawURL string) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; VeriNews/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing url: %w", err)
	}

	// Readability strips boilerplate — same algo as browser reader mode
	article, err := readability.FromReader(bytes.NewReader(body), parsedURL)
	if err != nil {
		return nil, fmt.Errorf("readability: %w", err)
	}

	// goquery for metadata readability doesn't always capture
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("html parse: %w", err)
	}

	result := &Result{
		URL:       rawURL,
		RawHTML:   string(body),
		Content:   strings.TrimSpace(article.TextContent),
		Title:     article.Title,
		Author:    article.Byline,
		Publisher: article.SiteName,
	}

	// fill gaps from OG/meta tags
	if result.Title == "" {
		result.Title = firstMeta(doc, `meta[property="og:title"]`, `meta[name="twitter:title"]`)
	}
	if result.Author == "" {
		result.Author = firstMeta(doc, `meta[name="author"]`, `meta[name="article:author"]`)
	}
	if result.Publisher == "" {
		result.Publisher = firstMeta(doc, `meta[property="og:site_name"]`)
	}
	if result.Publisher == "" {
		result.Publisher = parsedURL.Hostname()
	}
	result.PublishedAt = extractPublishedAt(doc)

	return result, nil
}

// firstMeta returns the content attribute of the first matching selector.
func firstMeta(doc *goquery.Document, selectors ...string) string {
	for _, sel := range selectors {
		if val, exists := doc.Find(sel).Attr("content"); exists && val != "" {
			return val
		}
	}
	return ""
}

var timeFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05-0700",
	"2006-01-02",
}

func extractPublishedAt(doc *goquery.Document) *time.Time {
	candidates := []string{
		doc.Find(`meta[property="article:published_time"]`).AttrOr("content", ""),
		doc.Find(`meta[property="og:published_time"]`).AttrOr("content", ""),
		doc.Find(`time[datetime]`).First().AttrOr("datetime", ""),
	}
	for _, s := range candidates {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		for _, layout := range timeFormats {
			if t, err := time.Parse(layout, s); err == nil {
				return &t
			}
		}
	}
	return nil
}

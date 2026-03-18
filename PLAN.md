# VeriNews — Build Plan (Go)

## What it is
A **philosophical truth engine** — not a fact-checker. Users submit a news article URL. The app scrapes it, runs 4 philosopher analysis passes via LLM, finds opposing coverage, and synthesizes a truth confidence rating.

---

## Tech Stack

| Layer | Choice | Reason |
|---|---|---|
| Language | Go 1.23+ | Single binary, real parallelism, clean CLI+web duality |
| HTTP | `chi` | Lightweight, idiomatic middleware |
| Database | PostgreSQL (Railway add-on) | Relational, JSONB support |
| Queue | `riverqueue/river` | Postgres-native, no Redis needed |
| Migrations | `pressly/goose` | Simple, SQL-based |
| DB queries | `sqlc` | Type-safe Go generated from SQL |
| CLI | `cobra` + `viper` | Subcommand routing, `.env` config |
| AI | OpenAI `gpt-4o-mini` | `sashabaranov/go-openai` |
| News Search | NewsAPI.org | Opposition article discovery |
| Frontend | `html/template` + HTMX | Or JSON API + separate frontend later |
| Hosting | Railway | 2 services: `app`, `worker` |

---

## Project Structure

```
truthseeker/
├── cmd/
│   └── truthseeker/
│       └── main.go              # cobra root, subcommands: web / worker / analyze
├── internal/
│   ├── config/
│   │   └── config.go            # viper config struct
│   ├── db/
│   │   ├── migrations/          # goose SQL migration files
│   │   ├── queries/             # sqlc .sql query files
│   │   └── generated/           # sqlc generated Go code (do not edit)
│   ├── scraper/
│   │   └── scraper.go           # fetch + clean article content
│   ├── analyzer/
│   │   ├── analyzer.go          # PhilosophicalAnalyzer interface
│   │   ├── socratic.go
│   │   ├── aristotelian.go
│   │   ├── platonic.go
│   │   └── stoic.go
│   ├── pipeline/
│   │   ├── scrape.go            # ScrapeJob
│   │   ├── claims.go            # ExtractClaimsJob
│   │   ├── analysis.go          # PhilosophicalAnalysisJob
│   │   ├── opposition.go        # OppositionDiscoveryJob
│   │   └── synthesis.go         # TruthSynthesisJob
│   ├── jobs/
│   │   └── registry.go          # river worker registration
│   ├── api/
│   │   ├── router.go
│   │   └── handlers.go
│   └── web/
│       └── templates/           # html/template files
├── Dockerfile
├── .env.example
├── go.mod
└── go.sum
```

---

## Entrypoint / Subcommands

```go
// cm./verinews/main.go
// cobra subcommands:

verinews web                         // start HTTP server
verinews worker                      // start River queue worker
verinews analyze <url>               // run pipeline inline (sync), print to stdout
verinews analyze <url> --format=json // JSON output
verinews migrate up                  // run goose migrations
verinews migrate down
```

`web` and `worker` are the two Railway services. `analyze` is the CLI path — runs the full pipeline synchronously in-process, no queue needed.

---

## Railway Services

```
project: truthseeker
  ├── app       START_COMMAND: ./verinews web
  ├── worker    START_COMMAND: ./verinews worker
  └── postgres  (Railway add-on)
```

Same binary, different start command. No Redis. No scheduler service (use River's built-in periodic jobs).

### Dockerfile

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o truthseeker ./cm./verinews

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /ap./verinews /usr/local/bin/
CMD ["truthseeker", "web"]
```

---

## Environment Variables

```env
DATABASE_URL=postgres://...
OPENAI_API_KEY=...
NEWS_API_KEY=...
APP_ENV=production        # or "local"
APP_PORT=8080
```

No `REDIS_URL` — River uses the same `DATABASE_URL`.

---

## Database Schema

### articles
```sql
id            BIGSERIAL PRIMARY KEY,
url           TEXT UNIQUE NOT NULL,
url_hash      TEXT UNIQUE NOT NULL,   -- SHA256 for fast dedup lookup
title         TEXT,
publisher     TEXT,
author        TEXT,
published_at  TIMESTAMPTZ,
raw_content   TEXT,
cleaned_content TEXT,
status        TEXT NOT NULL DEFAULT 'queued',  -- queued|processing|complete|failed
truth_score   FLOAT,
created_at    TIMESTAMPTZ DEFAULT NOW()
```

### claims
```sql
id                BIGSERIAL PRIMARY KEY,
article_id        BIGINT REFERENCES articles(id),
claim_text        TEXT NOT NULL,
claim_type        TEXT,               -- factual|opinion|predictive
evidence_present  BOOLEAN,
confidence_score  FLOAT,
created_at        TIMESTAMPTZ DEFAULT NOW()
```

### analyses
```sql
id               BIGSERIAL PRIMARY KEY,
article_id       BIGINT REFERENCES articles(id),
philosopher_mode TEXT NOT NULL,       -- socratic|aristotelian|platonic|stoic
json_output      JSONB NOT NULL,
completed_at     TIMESTAMPTZ DEFAULT NOW()
```

### related_articles
```sql
id                        BIGSERIAL PRIMARY KEY,
article_id                BIGINT REFERENCES articles(id),
url                       TEXT NOT NULL,
title                     TEXT,
publisher                 TEXT,
snippet                   TEXT,
relation_type             TEXT,       -- opposing|supporting|neutral
narrative_distance_score  FLOAT,
claim_contradiction_score FLOAT
```

### source_credibility
```sql
id           BIGSERIAL PRIMARY KEY,
domain       TEXT UNIQUE NOT NULL,
trust_score  FLOAT,
bias_rating  TEXT,
notes        TEXT
```

---

## Philosopher Interface

```go
type AnalysisResult struct {
    Mode    string          `json:"mode"`
    Output  json.RawMessage `json:"output"`
}

type PhilosophicalAnalyzer interface {
    Mode()    string
    Analyze(ctx context.Context, article Article, claims []Claim) (AnalysisResult, error)
}
```

Implementations: `SocraticAnalyzer`, `AristotelianAnalyzer`, `PlatonicAnalyzer`, `StoicAnalyzer` — all registered in `PhilosopherRegistry`.

---

## Job Pipeline

```
URL submitted (HTTP or CLI)
    ↓
ScrapeJob           — fetch, strip boilerplate, extract metadata
    ↓
ExtractClaimsJob    — OpenAI → structured JSON claims
    ↓
AnalysisJob         — goroutines: all 4 philosophers in parallel
    ↓
OppositionJob       — NewsAPI → fetch + scrape 3–5 articles
    ↓
SynthesisJob        — convergence scoring → truth_score written to DB
    ↓
article.status = "complete"
```

### Parallel philosopher execution (AnalysisJob)

```go
analyzers := registry.All()
results := make([]AnalysisResult, len(analyzers))
errs := make([]error, len(analyzers))

var wg sync.WaitGroup
for i, a := range analyzers {
    wg.Add(1)
    go func(i int, a PhilosophicalAnalyzer) {
        defer wg.Done()
        results[i], errs[i] = a.Analyze(ctx, article, claims)
    }(i, a)
}
wg.Wait()
```

### River job chaining

Each job inserts the next job on completion — River handles this atomically within the same Postgres transaction:

```go
func (j *ScrapeJob) Work(ctx context.Context, job *river.Job[ScrapeArgs]) error {
    // ... scrape logic ...
    _, err = riverClient.InsertTx(ctx, tx, ExtractClaimsArgs{ArticleID: articleID}, nil)
    return err
}
```

---

## Truth Synthesis Scoring

```
truth_score = weighted average of:
  claim_convergence_score   — related articles confirm same facts
  evidence_depth_score      — claims backed by cited evidence
  logic_integrity_score     — Aristotelian fallacy count, inverted
  source_credibility_score  — domain trust scores
  narrative_distance_score  — diversity of opposing views
```

| Range | Label |
|---|---|
| 0–30 | Highly Contested |
| 31–60 | Disputed |
| 61–80 | Mostly Supported |
| 81–100 | Well-Supported |

---

## HTTP API

```
POST /analyze              { "url": "..." } → { "article_id": 123 }
GET  /articles/:id         → article status + results (poll or SSE)
GET  /articles/:id/stream  → SSE for live pipeline progress
GET  /                     → submit form
GET  /articles/:id/report  → rendered report page
```

---

## Key Design Guardrails

- **Always async in web mode** — HTTP returns `article_id` immediately; client polls or listens on SSE
- **CLI is sync** — `analyze` subcommand runs pipeline inline, blocks until complete, prints to stdout
- **Dedup by URL hash** — SHA256 the URL, skip re-analysis if `status=complete`
- **Token trimming** — cap cleaned content at ~3000 tokens before any LLM call
- **Parallel philosophers** — all 4 analysis passes run concurrently in `AnalysisJob`
- **Isolated failures** — each River job retries independently; partial results are preserved
- **No Redis** — River uses Postgres exclusively

---

## AI Cost Estimate (MVP Scale)

| Item | Cost |
|---|---|
| Railway (app + worker + postgres) | ~$10–$25/month |
| OpenAI per analysis | ~$0.05–$0.25 |
| 100 articles/day | ~$150–$600/month AI |

Mitigate: token trimming, result caching, free tier rate limiting (5/day per IP).

---

## Phase 1 MVP

| Week | Goal |
|---|---|
| 1 | Scaffold: cobra CLI, config, goose migrations, sqlc setup, River wired |
| 2 | ScrapeJob + ExtractClaimsJob + basic web submit → poll flow |
| 3 | SocraticAnalyzer + AristotelianAnalyzer + report page |
| 4 | OppositionDiscoveryJob + basic convergence score |
| 5 | CLI `analyze` command, polish, rate limiting, Railway deploy |

## Phase 2

- PlatonicAnalyzer + StoicAnalyzer
- Source credibility seed data
- Full truth scoring weights
- SSE live pipeline progress
- Auth + tiered usage

## Phase 3

- Premium reports / API for journalists
- Browser extension
- "Philosophical Media Literacy" subscription

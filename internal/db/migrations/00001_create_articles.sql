-- +goose Up
CREATE TABLE articles (
    id              BIGSERIAL PRIMARY KEY,
    url             TEXT UNIQUE NOT NULL,
    url_hash        TEXT UNIQUE NOT NULL,
    title           TEXT,
    publisher       TEXT,
    author          TEXT,
    published_at    TIMESTAMPTZ,
    raw_content     TEXT,
    cleaned_content TEXT,
    status          TEXT NOT NULL DEFAULT 'queued',
    truth_score     FLOAT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_articles_status ON articles(status);

-- +goose Down
DROP TABLE articles;

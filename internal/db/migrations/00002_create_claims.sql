-- +goose Up
CREATE TABLE claims (
    id               BIGSERIAL PRIMARY KEY,
    article_id       BIGINT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    claim_text       TEXT NOT NULL,
    claim_type       TEXT,
    evidence_present BOOLEAN,
    confidence_score FLOAT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_claims_article_id ON claims(article_id);

-- +goose Down
DROP TABLE claims;

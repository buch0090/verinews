-- +goose Up
CREATE TABLE analyses (
    id               BIGSERIAL PRIMARY KEY,
    article_id       BIGINT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    philosopher_mode TEXT NOT NULL,
    json_output      JSONB NOT NULL,
    completed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_analyses_article_mode ON analyses(article_id, philosopher_mode);

-- +goose Down
DROP TABLE analyses;

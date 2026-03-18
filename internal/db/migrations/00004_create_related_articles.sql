-- +goose Up
CREATE TABLE related_articles (
    id                        BIGSERIAL PRIMARY KEY,
    article_id                BIGINT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    url                       TEXT NOT NULL,
    title                     TEXT,
    publisher                 TEXT,
    snippet                   TEXT,
    relation_type             TEXT,
    narrative_distance_score  FLOAT,
    claim_contradiction_score FLOAT
);

CREATE INDEX idx_related_articles_article_id ON related_articles(article_id);

-- +goose Down
DROP TABLE related_articles;

-- +goose Up
CREATE TABLE source_credibility (
    id          BIGSERIAL PRIMARY KEY,
    domain      TEXT UNIQUE NOT NULL,
    trust_score FLOAT,
    bias_rating TEXT,
    notes       TEXT
);

-- +goose Down
DROP TABLE source_credibility;

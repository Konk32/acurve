-- +goose Up
-- +goose StatementBegin

CREATE TABLE sources (
    id              SERIAL PRIMARY KEY,
    kind            TEXT NOT NULL CHECK (kind IN ('rss', 'youtube', 'reddit')),
    url             TEXT NOT NULL,
    name            TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    scrape_interval INTERVAL NOT NULL DEFAULT '3 hours',
    last_scraped_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (kind, url)
);

CREATE TABLE items (
    id              BIGSERIAL PRIMARY KEY,
    source_id       INTEGER NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    external_id     TEXT NOT NULL,
    url             TEXT NOT NULL,
    title           TEXT NOT NULL,
    author          TEXT,
    published_at    TIMESTAMPTZ,
    raw_content     TEXT,
    captions        TEXT,
    top_comments    JSONB,
    fetched_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_id, external_id)
);

CREATE INDEX idx_items_fetched_at ON items (fetched_at DESC);

CREATE TABLE summaries (
    item_id         BIGINT PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
    summary         TEXT NOT NULL,
    category        TEXT NOT NULL,
    score           INTEGER NOT NULL,
    reasoning       TEXT,
    model_used      TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_summaries_score ON summaries (score DESC);

CREATE TABLE digests (
    id              SERIAL PRIMARY KEY,
    sent_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivery_target TEXT NOT NULL,
    item_ids        BIGINT[] NOT NULL,
    success         BOOLEAN NOT NULL
);

-- Seed sources
INSERT INTO sources (kind, url, name) VALUES
    ('rss', 'https://news.ycombinator.com/rss', 'Hacker News'),
    ('rss', 'https://lobste.rs/rss', 'Lobsters'),
    ('rss', 'https://kubernetes.io/feed.xml', 'Kubernetes Blog'),
    ('rss', 'https://www.talos.dev/feed/', 'Talos Linux'),
    ('rss', 'https://fluxcd.io/index.xml', 'Flux CD'),
    ('rss', 'https://blog.golang.org/feed.atom', 'Go Blog'),
    ('reddit', 'https://www.reddit.com/r/kubernetes/new.json?limit=25', 'r/kubernetes'),
    ('reddit', 'https://www.reddit.com/r/selfhosted/new.json?limit=25', 'r/selfhosted'),
    ('reddit', 'https://www.reddit.com/r/homelab/new.json?limit=25', 'r/homelab'),
    ('reddit', 'https://www.reddit.com/r/golang/new.json?limit=25', 'r/golang');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS digests;
DROP TABLE IF EXISTS summaries;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS sources;
-- +goose StatementEnd

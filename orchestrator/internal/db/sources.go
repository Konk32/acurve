package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

type Source struct {
	ID             int        `json:"id"`
	Kind           string     `json:"kind"`
	URL            string     `json:"url"`
	Name           string     `json:"name"`
	Enabled        bool       `json:"enabled"`
	ScrapeInterval string     `json:"scrape_interval"`
	LastScrapedAt  *time.Time `json:"last_scraped_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) GetSource(ctx context.Context, id int) (Source, error) {
	var src Source
	err := s.pool.QueryRow(ctx, `
		SELECT id, kind, url, name, enabled,
		       scrape_interval::text, last_scraped_at, created_at
		FROM sources WHERE id = $1
	`, id).Scan(
		&src.ID, &src.Kind, &src.URL, &src.Name, &src.Enabled,
		&src.ScrapeInterval, &src.LastScrapedAt, &src.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Source{}, fmt.Errorf("get source %d: %w", id, ErrNotFound)
		}
		return Source{}, fmt.Errorf("get source %d: %w", id, err)
	}
	return src, nil
}

func (s *Store) ListSources(ctx context.Context) ([]Source, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, kind, url, name, enabled,
		       scrape_interval::text, last_scraped_at, created_at
		FROM sources
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var src Source
		if err := rows.Scan(
			&src.ID, &src.Kind, &src.URL, &src.Name, &src.Enabled,
			&src.ScrapeInterval, &src.LastScrapedAt, &src.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

func (s *Store) CreateSource(ctx context.Context, kind, url, name string) (Source, error) {
	var src Source
	err := s.pool.QueryRow(ctx, `
		INSERT INTO sources (kind, url, name)
		VALUES ($1, $2, $3)
		RETURNING id, kind, url, name, enabled,
		          scrape_interval::text, last_scraped_at, created_at
	`, kind, url, name).Scan(
		&src.ID, &src.Kind, &src.URL, &src.Name, &src.Enabled,
		&src.ScrapeInterval, &src.LastScrapedAt, &src.CreatedAt,
	)
	if err != nil {
		return Source{}, fmt.Errorf("create source: %w", err)
	}
	return src, nil
}

type UpdateSourceParams struct {
	Enabled        *bool
	ScrapeInterval *string
}

func (s *Store) UpdateSource(ctx context.Context, id int, p UpdateSourceParams) (Source, error) {
	var src Source
	err := s.pool.QueryRow(ctx, `
		UPDATE sources SET
			enabled         = COALESCE($2, enabled),
			scrape_interval = COALESCE($3::interval, scrape_interval)
		WHERE id = $1
		RETURNING id, kind, url, name, enabled,
		          scrape_interval::text, last_scraped_at, created_at
	`, id, p.Enabled, p.ScrapeInterval).Scan(
		&src.ID, &src.Kind, &src.URL, &src.Name, &src.Enabled,
		&src.ScrapeInterval, &src.LastScrapedAt, &src.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Source{}, fmt.Errorf("update source %d: %w", id, ErrNotFound)
		}
		return Source{}, fmt.Errorf("update source %d: %w", id, err)
	}
	return src, nil
}

func (s *Store) DeleteSource(ctx context.Context, id int) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete source %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delete source %d: %w", id, ErrNotFound)
	}
	return nil
}

package db

import (
	"context"
	"fmt"
	"time"
)

// DigestItem is a scored item that is eligible for inclusion in a digest.
type DigestItem struct {
	ItemID      int64
	Title       string
	URL         string
	Summary     string
	Category    string
	Score       int
	SourceName  string
	PublishedAt *time.Time
}

// GetDigestItems returns scored items that have not been included in any
// previous digest and whose score is >= minScore. Results are ordered by
// category, then score descending.
func (s *Store) GetDigestItems(ctx context.Context, minScore int) ([]DigestItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			i.id,
			i.title,
			i.url,
			sm.summary,
			sm.category,
			sm.score,
			src.name  AS source_name,
			i.published_at
		FROM items i
		JOIN summaries sm ON sm.item_id = i.id
		JOIN sources   src ON src.id = i.source_id
		WHERE sm.score >= $1
		  AND i.id NOT IN (
		      SELECT UNNEST(item_ids) FROM digests WHERE success = TRUE
		  )
		ORDER BY sm.category, sm.score DESC
	`, minScore)
	if err != nil {
		return nil, fmt.Errorf("get digest items: %w", err)
	}
	defer rows.Close()

	var items []DigestItem
	for rows.Next() {
		var it DigestItem
		if err := rows.Scan(
			&it.ItemID, &it.Title, &it.URL,
			&it.Summary, &it.Category, &it.Score,
			&it.SourceName, &it.PublishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan digest item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// InsertDigest records a sent digest.
func (s *Store) InsertDigest(ctx context.Context, target string, itemIDs []int64, success bool) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO digests (delivery_target, item_ids, success)
		VALUES ($1, $2, $3)
	`, target, itemIDs, success)
	if err != nil {
		return fmt.Errorf("insert digest: %w", err)
	}
	return nil
}

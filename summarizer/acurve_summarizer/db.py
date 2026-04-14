"""Database helpers for the summarizer."""

from __future__ import annotations

import psycopg
import psycopg.rows
import structlog

log = structlog.get_logger()


def connect(db_url: str) -> psycopg.Connection:
    return psycopg.connect(db_url)


def get_unsummarised_items(conn: psycopg.Connection, batch_size: int = 50) -> list[dict]:
    """Return items that do not yet have a summary, oldest first."""
    with conn.cursor(row_factory=psycopg.rows.dict_row) as cur:
        cur.execute("""
            SELECT i.id, i.title, i.url, i.raw_content, i.captions, i.top_comments
            FROM items i
            LEFT JOIN summaries s ON s.item_id = i.id
            WHERE s.item_id IS NULL
            ORDER BY i.fetched_at ASC
            LIMIT %s
        """, (batch_size,))
        return cur.fetchall()


def insert_summary(
    conn: psycopg.Connection,
    *,
    item_id: int,
    summary: str,
    category: str,
    score: int,
    reasoning: str | None,
    model_used: str,
) -> None:
    with conn.cursor() as cur:
        cur.execute("""
            INSERT INTO summaries (item_id, summary, category, score, reasoning, model_used)
            VALUES (%s, %s, %s, %s, %s, %s)
            ON CONFLICT (item_id) DO NOTHING
        """, (item_id, summary, category, score, reasoning, model_used))

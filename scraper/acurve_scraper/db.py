"""Database helpers for the scraper."""

from __future__ import annotations

import json
from datetime import datetime
from typing import Any

import psycopg
import structlog

log = structlog.get_logger()


def connect(db_url: str) -> psycopg.Connection:
    return psycopg.connect(db_url)


def list_enabled_sources(conn: psycopg.Connection) -> list[dict]:
    """Return all enabled sources that are due for scraping."""
    with conn.cursor(row_factory=psycopg.rows.dict_row) as cur:
        cur.execute("""
            SELECT id, kind, url, name, scrape_interval, last_scraped_at
            FROM sources
            WHERE enabled = TRUE
              AND (last_scraped_at IS NULL
                   OR last_scraped_at + scrape_interval < NOW())
            ORDER BY last_scraped_at ASC NULLS FIRST
        """)
        return cur.fetchall()


def upsert_item(
    conn: psycopg.Connection,
    *,
    source_id: int,
    external_id: str,
    url: str,
    title: str,
    author: str | None = None,
    published_at: datetime | None = None,
    raw_content: str | None = None,
    captions: str | None = None,
    top_comments: list[dict] | None = None,
) -> int | None:
    """Insert a new item; return its id (or None if it already exists)."""
    with conn.cursor() as cur:
        cur.execute("""
            INSERT INTO items
                (source_id, external_id, url, title, author,
                 published_at, raw_content, captions, top_comments)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)
            ON CONFLICT (source_id, external_id) DO NOTHING
            RETURNING id
        """, (
            source_id, external_id, url, title, author,
            published_at, raw_content, captions,
            json.dumps(top_comments) if top_comments else None,
        ))
        row = cur.fetchone()
        return row[0] if row else None


def mark_source_scraped(conn: psycopg.Connection, source_id: int) -> None:
    with conn.cursor() as cur:
        cur.execute(
            "UPDATE sources SET last_scraped_at = NOW() WHERE id = %s",
            (source_id,),
        )

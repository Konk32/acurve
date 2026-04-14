"""RSS / Atom feed scraper."""

from __future__ import annotations

from datetime import datetime, timezone
from email.utils import parsedate_to_datetime

import feedparser
import structlog

log = structlog.get_logger()


def _parse_date(entry: feedparser.FeedParserDict) -> datetime | None:
    """Return a timezone-aware datetime from the entry, or None."""
    if hasattr(entry, "published_parsed") and entry.published_parsed:
        return datetime(*entry.published_parsed[:6], tzinfo=timezone.utc)
    if hasattr(entry, "updated_parsed") and entry.updated_parsed:
        return datetime(*entry.updated_parsed[:6], tzinfo=timezone.utc)
    return None


def scrape(url: str) -> list[dict]:
    """
    Fetch and parse an RSS/Atom feed.

    Returns a list of dicts with keys:
        external_id, url, title, author, published_at, raw_content
    """
    log.info("scraping rss feed", url=url)
    feed = feedparser.parse(url)

    if feed.bozo and not feed.entries:
        log.warning("feed parse error", url=url, exc=feed.bozo_exception)
        return []

    items = []
    for entry in feed.entries:
        external_id = getattr(entry, "id", None) or entry.get("link", "")
        if not external_id:
            continue

        content = ""
        if entry.get("content"):
            content = entry.content[0].value
        elif entry.get("summary"):
            content = entry.summary

        items.append({
            "external_id": external_id,
            "url": entry.get("link", ""),
            "title": entry.get("title", ""),
            "author": entry.get("author"),
            "published_at": _parse_date(entry),
            "raw_content": content or None,
        })

    log.info("rss feed scraped", url=url, count=len(items))
    return items

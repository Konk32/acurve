"""Reddit scraper using the public JSON API (no auth needed for read-only)."""

from __future__ import annotations

from datetime import datetime, timezone

import httpx
import structlog

log = structlog.get_logger()

_HEADERS = {"User-Agent": "acurve-scraper/0.1 (by /u/acurve_bot)"}


def scrape(url: str) -> list[dict]:
    """
    Fetch a subreddit's new posts via the public JSON API.

    URL format: https://www.reddit.com/r/{sub}/new.json?limit=25
    """
    log.info("scraping reddit", url=url)
    try:
        resp = httpx.get(url, headers=_HEADERS, timeout=15, follow_redirects=True)
        resp.raise_for_status()
    except httpx.HTTPError as exc:
        log.error("reddit fetch failed", url=url, exc=str(exc))
        return []

    data = resp.json()
    posts = data.get("data", {}).get("children", [])

    items = []
    for child in posts:
        post = child.get("data", {})
        post_id = post.get("id")
        if not post_id:
            continue

        # Combine title + selftext as content
        selftext = post.get("selftext", "")
        content = selftext[:4000] if selftext else None

        # Collect top comments (not fetched here — would require extra request)
        published_at = datetime.fromtimestamp(post["created_utc"], tz=timezone.utc) if post.get("created_utc") else None

        items.append({
            "external_id": post_id,
            "url": f"https://www.reddit.com{post.get('permalink', '')}",
            "title": post.get("title", ""),
            "author": post.get("author"),
            "published_at": published_at,
            "raw_content": content,
        })

    log.info("reddit scraped", url=url, count=len(items))
    return items

"""
acurve scraper — entry point.

Fetches all enabled sources that are due for scraping and writes items to Postgres.
Exits with code 0 on success, 1 on fatal error.
"""

from __future__ import annotations

import sys

import structlog

from . import config, db
from .sources import rss, youtube, reddit

log = structlog.get_logger()


def _scrape_source(source: dict) -> list[dict]:
    kind = source["kind"]
    url = source["url"]
    if kind == "rss":
        return rss.scrape(url)
    if kind == "youtube":
        return youtube.scrape(url)
    if kind == "reddit":
        return reddit.scrape(url)
    log.warning("unknown source kind", kind=kind)
    return []


def main() -> None:
    structlog.configure(
        wrapper_class=structlog.make_filtering_bound_logger(20),  # INFO
        processors=[
            structlog.processors.TimeStamper(fmt="iso"),
            structlog.processors.JSONRenderer(),
        ],
    )

    log.info("scraper starting")

    try:
        conn = db.connect(config.DB_URL)
    except Exception as exc:
        log.error("db connection failed", exc=str(exc))
        sys.exit(1)

    try:
        sources = db.list_enabled_sources(conn)
        log.info("sources to scrape", count=len(sources))

        total_new = 0
        for source in sources:
            log.info("scraping source", name=source["name"], kind=source["kind"])
            try:
                items = _scrape_source(source)
            except Exception as exc:
                log.error("scrape failed", source=source["name"], exc=str(exc))
                continue

            new_count = 0
            for item in items:
                try:
                    item_id = db.upsert_item(
                        conn,
                        source_id=source["id"],
                        **item,
                    )
                    if item_id is not None:
                        new_count += 1
                except Exception as exc:
                    log.error("upsert failed", title=item.get("title"), exc=str(exc))

            conn.commit()
            db.mark_source_scraped(conn, source["id"])
            conn.commit()
            total_new += new_count
            log.info("source done", name=source["name"], new=new_count)

        log.info("scraper finished", total_new=total_new)

    finally:
        conn.close()


if __name__ == "__main__":
    main()

"""
acurve summarizer — entry point.

Picks up unsummarised items from Postgres, calls the LLM, and writes
summaries back. Designed to run to completion and exit (CronJob / one-shot).

Environment variables
---------------------
DB_URL              (required) PostgreSQL connection string
LLM_PROVIDER        "anthropic" (default) or "ollama"

Anthropic provider:
  ANTHROPIC_API_KEY  (required)
  ANTHROPIC_MODEL    default: claude-haiku-4-5-20251001

Ollama provider:
  OLLAMA_URL         (required) e.g. http://10.0.0.1:11434
  OLLAMA_MODEL       default: gemma3:12b

Common:
  BATCH_SIZE         items per run, default 50
"""

from __future__ import annotations

import os
import sys

import structlog

from . import db
from .llm import summarise

log = structlog.get_logger()


def _require(key: str) -> str:
    val = os.environ.get(key)
    if not val:
        log.error("missing required env var", key=key)
        sys.exit(1)
    return val


def main() -> None:
    structlog.configure(
        wrapper_class=structlog.make_filtering_bound_logger(20),  # INFO
        processors=[
            structlog.processors.TimeStamper(fmt="iso"),
            structlog.processors.JSONRenderer(),
        ],
    )

    db_url = _require("DB_URL")
    provider = os.environ.get("LLM_PROVIDER", "anthropic")
    batch_size = int(os.environ.get("BATCH_SIZE", "20"))

    # Resolve provider-specific config.
    api_key: str | None = None
    ollama_url: str | None = None

    if provider == "anthropic":
        api_key = _require("ANTHROPIC_API_KEY")
        model = os.environ.get("ANTHROPIC_MODEL", "claude-haiku-4-5-20251001")
    elif provider == "ollama":
        ollama_url = _require("OLLAMA_URL")
        model = os.environ.get("OLLAMA_MODEL", "gemma3:12b")
    else:
        log.error("unknown LLM_PROVIDER", provider=provider)
        sys.exit(1)

    log.info("summarizer starting", provider=provider, model=model, batch_size=batch_size)

    try:
        conn = db.connect(db_url)
    except Exception as exc:
        log.error("db connection failed", exc=str(exc))
        sys.exit(1)

    try:
        items = db.get_unsummarised_items(conn, batch_size=batch_size)
        log.info("items to summarise", count=len(items))

        if not items:
            log.info("nothing to do")
            return

        ok = 0
        fail = 0
        for item in items:
            item_id = item["id"]
            title = item["title"]
            log.info("summarising", item_id=item_id, title=title[:80])

            try:
                result = summarise(
                    title=title,
                    url=item["url"],
                    raw_content=item.get("raw_content"),
                    captions=item.get("captions"),
                    top_comments=item.get("top_comments"),
                    provider=provider,
                    model=model,
                    api_key=api_key,
                    ollama_url=ollama_url,
                )
            except Exception as exc:
                log.error("llm call failed", item_id=item_id, exc=str(exc))
                fail += 1
                continue

            try:
                db.insert_summary(
                    conn,
                    item_id=item_id,
                    summary=result["summary"],
                    category=result["category"],
                    score=result["score"],
                    reasoning=result.get("reasoning"),
                    model_used=model,
                )
                conn.commit()
                ok += 1
                log.info(
                    "summarised",
                    item_id=item_id,
                    score=result["score"],
                    category=result["category"],
                )
            except Exception as exc:
                log.error("db insert failed", item_id=item_id, exc=str(exc))
                conn.rollback()
                fail += 1

        log.info("summarizer finished", ok=ok, fail=fail)

    finally:
        conn.close()


if __name__ == "__main__":
    main()

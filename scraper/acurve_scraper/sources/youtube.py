"""YouTube channel scraper (RSS feed + transcript API, no Data API key needed)."""

from __future__ import annotations

from datetime import datetime, timezone

import structlog
from youtube_transcript_api import YouTubeTranscriptApi, NoTranscriptFound, TranscriptsDisabled

from . import rss

log = structlog.get_logger()

_TRANSCRIPT_LANGS = ["en", "en-US", "en-GB"]


def _get_captions(video_id: str) -> str | None:
    try:
        transcript = YouTubeTranscriptApi.get_transcript(video_id, languages=_TRANSCRIPT_LANGS)
        text = " ".join(seg["text"] for seg in transcript)
        # Truncate to avoid huge LLM prompts
        return text[:8000] if len(text) > 8000 else text
    except (NoTranscriptFound, TranscriptsDisabled):
        return None
    except Exception as exc:
        log.warning("transcript fetch failed", video_id=video_id, exc=str(exc))
        return None


def scrape(url: str) -> list[dict]:
    """
    Scrape a YouTube channel RSS feed and enrich items with captions.

    URL format: https://www.youtube.com/feeds/videos.xml?channel_id=CHANNEL_ID
    """
    items = rss.scrape(url)
    for item in items:
        # YouTube entry id is like: yt:video:VIDEO_ID
        raw_id = item["external_id"]
        video_id = raw_id.split(":")[-1] if ":" in raw_id else raw_id
        item["captions"] = _get_captions(video_id)
        log.debug("youtube item", video_id=video_id, has_captions=item["captions"] is not None)
    return items

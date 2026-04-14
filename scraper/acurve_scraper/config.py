"""Configuration loaded from environment variables."""

import os


def _require(key: str) -> str:
    val = os.environ.get(key)
    if not val:
        raise RuntimeError(f"Missing required environment variable: {key}")
    return val


def _optional(key: str, default: str = "") -> str:
    return os.environ.get(key, default)


DB_URL: str = _require("DB_URL")

# Reddit (optional — only needed if reddit sources exist)
REDDIT_CLIENT_ID: str = _optional("REDDIT_CLIENT_ID")
REDDIT_CLIENT_SECRET: str = _optional("REDDIT_CLIENT_SECRET")
REDDIT_USER_AGENT: str = _optional("REDDIT_USER_AGENT", "acurve-scraper/0.1")

# YouTube (optional — captions via youtube-transcript-api need no key)
YOUTUBE_API_KEY: str = _optional("YOUTUBE_API_KEY")

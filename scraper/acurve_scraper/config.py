"""Configuration loaded from environment variables."""

import os


def _require(key: str) -> str:
    val = os.environ.get(key)
    if not val:
        raise RuntimeError(f"Missing required environment variable: {key}")
    return val


DB_URL: str = _require("DB_URL")

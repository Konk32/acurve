FROM python:3.12-slim AS builder

WORKDIR /build

RUN pip install --no-cache-dir uv

COPY scraper/pyproject.toml ./
RUN uv pip install --system --no-cache .

# ---
FROM python:3.12-slim

RUN useradd --no-create-home --shell /sbin/nologin scraper

WORKDIR /app
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY scraper/acurve_scraper ./acurve_scraper

USER scraper

ENTRYPOINT ["python", "-m", "acurve_scraper"]

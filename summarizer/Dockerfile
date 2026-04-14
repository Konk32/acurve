FROM python:3.12-slim AS builder

WORKDIR /build

RUN pip install --no-cache-dir uv

COPY summarizer/pyproject.toml ./
RUN uv pip install --system --no-cache .

# ---
FROM python:3.12-slim

RUN useradd --no-create-home --shell /sbin/nologin summarizer

WORKDIR /app
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY summarizer/acurve_summarizer ./acurve_summarizer

USER summarizer

ENTRYPOINT ["python", "-m", "acurve_summarizer"]

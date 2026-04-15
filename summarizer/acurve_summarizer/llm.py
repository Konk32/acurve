"""
Pluggable LLM backend for summarisation.

Providers
---------
anthropic (default)
    Calls the Anthropic Messages API with prompt caching on the system prompt.
    Requires: ANTHROPIC_API_KEY, ANTHROPIC_MODEL (default claude-haiku-4-5-20251001)

ollama
    Calls a local Ollama instance via its REST API.  Useful when the Anthropic
    API is unavailable or for cost-free local testing with Gemma 3 / other models.
    Requires: OLLAMA_URL (e.g. http://10.0.0.1:11434), OLLAMA_MODEL (default gemma3:12b)
"""

from __future__ import annotations

import json
from typing import TypedDict

import anthropic
import httpx
import structlog

log = structlog.get_logger()

# ---------------------------------------------------------------------------
# System prompt — shared by both providers.
# ---------------------------------------------------------------------------
_SYSTEM_PROMPT = """
You are a technical news filter for an IT-focused developer and homelab enthusiast.

## Reader profile

The reader works with the following technologies daily:

**Infrastructure & orchestration**
- Kubernetes (specifically Talos Linux — a minimal, immutable OS designed for k8s)
- GitOps via Flux CD (HelmRelease, Kustomization, image automation)
- CNPG (CloudNativePG) for Postgres on Kubernetes
- Cert-manager, external-secrets, MetalLB, Cilium

**Observability**
- Prometheus + Alertmanager + Grafana stacks
- Loki for log aggregation
- OpenTelemetry

**Programming**
- Go (primary language — idiomatic, stdlib-first, slog, chi, pgx)
- Python (secondary — scripting, data processing, ML tooling)
- Bash / shell scripting

**Homelab & self-hosted**
- Proxmox VE for VM/container hypervisor
- Home Assistant for home automation (integrations, custom components)
- 3D printing (Bambu Lab, OrcaSlicer, calibration, filament)
- Self-hosted services: Vaultwarden, Nextcloud, Jellyfin, arr-stack, etc.

**Security & networking**
- WireGuard VPN (site-to-site and road-warrior)
- Cloudflare Tunnel / DNS
- SOPS for secret encryption
- Basic PKI / cert management

**AI / ML**
- Follows Anthropic, OpenAI, Google DeepMind closely
- Interested in local inference (Ollama, llama.cpp, vLLM)
- Particularly interested in AI coding tools, agents, and infrastructure

## Scoring rubric

Score 90–100 (critical / act now):
- Security vulnerabilities or breaking changes directly affecting tools the reader uses
  (Talos, Flux, Kubernetes, Go stdlib, Anthropic API, Home Assistant)
- Major releases with significant new features for those tools
- Deprecations or end-of-life notices for actively-used tools

Score 70–89 (important — include in digest):
- Significant releases for adjacent tools (Cilium, Prometheus, Grafana, CNPG, cert-manager)
- Notable CVEs or security advisories in the broader k8s / cloud-native ecosystem
- Interesting Go language evolution (proposals, accepted specs, standard library additions)
- Meaningful AI/ML research affecting practical inference or tooling
- Homelab-relevant hardware releases (low-power servers, NVMe, networking gear)
- Thoughtful technical blog posts or conference talks on distributed systems, observability,
  or Kubernetes internals

Score 50–69 (interesting but tangential):
- General cloud-native news not directly involving reader's stack
- Python ecosystem updates (useful but not the reader's primary language)
- Peripheral AI/ML news (academic papers with limited practical impact near-term)
- Homelab community builds that are impressive but not actionable

Score 0–49 (skip — do not include):
- Marketing content, press releases, sponsored posts, vendor announcements without
  substance
- Duplicate stories covering the same event from a different angle
- Social media drama, opinion pieces without technical merit
- Topics entirely unrelated to tech (finance news, sports, politics, etc.)
- Clickbait headlines with thin content

## Output format

Return ONLY a JSON object with these exact keys. No markdown fences, no preamble.

{
  "summary": "2–4 sentence TL;DR, concrete and specific. State what changed, why it matters, and
              any action the reader should take. No marketing language. No 'this is a blog post
              about...' framing — just the facts.",
  "category": "one of: kubernetes | ai | security | go | homelab | hardware | tooling | other",
  "score": <integer 0–100>,
  "reasoning": "one sentence explaining the score relative to the reader's interests"
}

Category guide:
- kubernetes  → Kubernetes, Talos, Flux, Helm, k8s-adjacent CNI/CSI/operators
- ai          → LLMs, ML infrastructure, inference, AI coding tools, Anthropic/OpenAI/etc.
- security    → CVEs, auth, cryptography, supply chain, network security
- go          → Go language, stdlib, popular Go tools/frameworks
- homelab     → self-hosted, Proxmox, Home Assistant, NAS, home networking
- hardware    → CPUs, GPUs, storage, SBCs, 3D printing hardware
- tooling     → CLI tools, developer experience, observability, CI/CD (not kubernetes-specific)
- other       → anything that doesn't fit above

If content is empty or too short to summarise meaningfully, return score 0 with a one-sentence
summary noting the lack of content.
""".strip()


class Summary(TypedDict):
    summary: str
    category: str
    score: int
    reasoning: str


def summarise(
    *,
    title: str,
    url: str,
    raw_content: str | None,
    captions: str | None,
    top_comments: list | None,
    provider: str,
    model: str,
    api_key: str | None = None,
    ollama_url: str | None = None,
) -> Summary:
    """
    Summarise a single item using the configured LLM provider.

    provider="anthropic": uses Anthropic Messages API (api_key required)
    provider="ollama":    uses local Ollama REST API (ollama_url required)
    """
    user_content = _build_user_content(title, url, raw_content, captions, top_comments)

    if provider == "anthropic":
        if not api_key:
            raise ValueError("api_key is required for the anthropic provider")
        return _summarise_anthropic(user_content, model=model, api_key=api_key)
    elif provider == "ollama":
        if not ollama_url:
            raise ValueError("ollama_url is required for the ollama provider")
        return _summarise_ollama(user_content, model=model, ollama_url=ollama_url)
    else:
        raise ValueError(f"unknown provider {provider!r} — expected 'anthropic' or 'ollama'")


# ---------------------------------------------------------------------------
# Internals
# ---------------------------------------------------------------------------

def _build_user_content(
    title: str,
    url: str,
    raw_content: str | None,
    captions: str | None,
    top_comments: list | None,
) -> str:
    parts = [f"Title: {title}", f"URL: {url}"]

    if raw_content:
        parts.append(f"Content:\n{raw_content[:6000]}")

    if captions:
        parts.append(f"Captions (YouTube transcript):\n{captions[:4000]}")

    if top_comments:
        comments_text = "\n".join(
            f"- [{c.get('score', 0)}] {c.get('author', '?')}: {c.get('text', '')[:300]}"
            for c in top_comments[:10]
        )
        parts.append(f"Top comments:\n{comments_text}")

    if len(parts) <= 2:
        parts.append("(no body content available)")

    return "\n\n".join(parts)


def _parse_json(text: str) -> Summary:
    """Parse the JSON blob returned by either provider."""
    try:
        data = json.loads(text)
    except json.JSONDecodeError:
        log.warning("failed to parse llm json", raw=text[:500])
        return Summary(
            summary="Parse error — raw response logged.",
            category="other",
            score=0,
            reasoning="LLM returned non-JSON output.",
        )

    try:
        score = max(0, min(100, int(data.get("score", 0))))
    except (TypeError, ValueError):
        score = 0

    return Summary(
        summary=str(data.get("summary", "")),
        category=str(data.get("category", "other")),
        score=score,
        reasoning=str(data.get("reasoning", "")),
    )


def _summarise_anthropic(user_content: str, *, model: str, api_key: str) -> Summary:
    """
    Call Anthropic Messages API with prompt caching on the system prompt.

    Minimum cacheable prefix for claude-haiku-4-5 is 2 048 tokens; this
    prompt is ~850 tokens, so caching activates on Sonnet/Opus tiers and
    is attempted (but may not activate) on Haiku.  Still worth including
    for correctness and future-proofing.
    """
    client = anthropic.Anthropic(api_key=api_key)

    response = client.messages.create(
        model=model,
        max_tokens=512,
        system=[
            {
                "type": "text",
                "text": _SYSTEM_PROMPT,
                "cache_control": {"type": "ephemeral"},
            }
        ],
        messages=[{"role": "user", "content": user_content}],
    )

    cache_read = response.usage.cache_read_input_tokens or 0
    cache_created = response.usage.cache_creation_input_tokens or 0
    log.debug(
        "anthropic usage",
        input_tokens=response.usage.input_tokens,
        output_tokens=response.usage.output_tokens,
        cache_read=cache_read,
        cache_created=cache_created,
    )

    text = next((b.text for b in response.content if b.type == "text"), "{}")
    return _parse_json(text)


def _summarise_ollama(user_content: str, *, model: str, ollama_url: str) -> Summary:
    """
    Call a local Ollama instance via its /api/chat endpoint.

    format="json" instructs Ollama to constrain the output to valid JSON,
    which removes the need for fence-stripping.
    """
    url = ollama_url.rstrip("/") + "/api/chat"

    payload = {
        "model": model,
        "stream": False,
        "format": "json",
        "messages": [
            {"role": "system", "content": _SYSTEM_PROMPT},
            {"role": "user", "content": user_content},
        ],
    }

    try:
        resp = httpx.post(url, json=payload, timeout=120.0)
        resp.raise_for_status()
    except httpx.HTTPError as exc:
        raise RuntimeError(f"ollama request failed: {exc}") from exc

    data = resp.json()
    text = data.get("message", {}).get("content", "{}")

    log.debug(
        "ollama usage",
        model=model,
        prompt_tokens=data.get("prompt_eval_count"),
        completion_tokens=data.get("eval_count"),
    )

    return _parse_json(text)

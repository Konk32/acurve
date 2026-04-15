# acurve — Setup Guide

Periodic IT-news digest: scrapes RSS, YouTube, and Reddit; summarizes with an LLM; delivers twice-daily to Discord. Runs on Kubernetes via Flux + bjw-s/app-template.

---

## Prerequisites

### Local tools

| Tool | Minimum version | Purpose |
|------|-----------------|---------|
| Go | 1.23 | Build the orchestrator |
| Python | 3.12 | Run the scraper / summarizer |
| [uv](https://github.com/astral-sh/uv) | any | Python package management |
| Docker + Docker Compose | v2 | Local development |
| [age](https://github.com/FiloSottile/age) | any | Encrypt secrets for Flux |
| [sops](https://github.com/getsops/sops) | 3.x | Encrypt/decrypt secret files |
| kubectl | any | Kubernetes access |
| [flux CLI](https://fluxcd.io/flux/installation/) | 2.x | Flux operations |

### External services

| Service | Required | Notes |
|---------|----------|-------|
| Postgres | Yes | Provided locally via Docker Compose; CNPG in Kubernetes |
| Anthropic API key | For `LLM_PROVIDER=anthropic` | `claude-haiku-4-5-20251001` is the default model |
| Ollama instance | For `LLM_PROVIDER=ollama` | Gemma 3 12B runs well on a 4080 16 GB |
| Discord webhook URL | For digest delivery | Create in Discord: Server Settings → Integrations → Webhooks |

### Kubernetes cluster (for production)

- Talos Linux (or any CNCF-conformant cluster)
- Flux CD installed and bootstrapped
- [CloudNativePG operator](https://cloudnative-pg.io/) (CNPG) installed
- nginx ingress controller
- SOPS age key registered in the cluster (see [Secrets setup](#secrets-setup))

---

## Local development

### 1. Clone and set up environment

```bash
git clone https://github.com/Konk32/acurve.git
cd acurve
```

Create a `.env` file (gitignored):

```bash
# Required for the summarizer
ANTHROPIC_API_KEY=sk-ant-...

# Or use Ollama instead
# LLM_PROVIDER=ollama
# OLLAMA_URL=http://10.0.0.1:11434

# Required for digest delivery
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/...
```

### 2. Start Postgres and the orchestrator

```bash
docker compose up
```

This starts:
- `postgres` on port 5432
- `orchestrator` on port 8080 (runs goose migrations automatically on startup)

Dashboard: http://localhost:8080  
Health check: http://localhost:8080/healthz

### 3. Run the scraper

```bash
docker compose --profile scrape run --rm scraper
```

Scrapes all enabled sources (RSS, Reddit feeds seeded by the migration) and inserts items into Postgres. Run it once to populate data.

### 4. Run the summarizer

**With Anthropic:**
```bash
ANTHROPIC_API_KEY=sk-ant-... \
  docker compose --profile summarize run --rm summarizer
```

**With Ollama (no API key needed):**
```bash
LLM_PROVIDER=ollama OLLAMA_URL=http://10.0.0.1:11434 \
  docker compose --profile summarize run --rm summarizer
```

The summarizer picks up all unsummarized items, scores them 0–100, and writes summaries back to Postgres.

### 5. Preview and send the digest

```bash
# Preview what would be in the next digest
curl http://localhost:8080/api/digest/preview

# Send the digest to Discord (DISCORD_WEBHOOK_URL must be set in the orchestrator container)
curl -X POST http://localhost:8080/api/digest/send
```

### 6. Manage sources

Use the dashboard at http://localhost:8080 or the REST API:

```bash
# List sources
curl http://localhost:8080/api/sources

# Add a source
curl -X POST http://localhost:8080/api/sources \
  -H 'Content-Type: application/json' \
  -d '{"kind":"rss","url":"https://example.com/feed","name":"Example Blog"}'

# Disable a source (id=1)
curl -X PATCH http://localhost:8080/api/sources/1 \
  -H 'Content-Type: application/json' \
  -d '{"enabled":false}'

# Delete a source
curl -X DELETE http://localhost:8080/api/sources/1
```

---

## Production deployment (Kubernetes / Flux)

### Secrets setup

**Generate an age key pair** (one-time):

```bash
age-keygen -o age.key
# Prints: Public key: age1abc123...
```

Store the **private** key in the cluster so Flux can decrypt at apply time:

```bash
kubectl create secret generic sops-age \
  --namespace=flux-system \
  --from-file=age.agekey=age.key
```

Keep `age.key` in a password manager and delete or move it off the repo directory — it is gitignored but treat it as a credential.

**Configure the repo's encryption rule** — edit [`.sops.yaml`](.sops.yaml) and replace the placeholder with the public key from above:

```yaml
creation_rules:
  - path_regex: deploy/base/secrets\.sops\.yaml$
    age: age1abc123...   # <-- your public key
```

**Fill in and encrypt secrets:**

```bash
# Edit deploy/base/secrets.sops.yaml — replace all CHANGE_ME values:
#   acurve-postgres-credentials: username + password
#   acurve-secrets: DB_URL, DISCORD_WEBHOOK_URL, ANTHROPIC_API_KEY (or OLLAMA_URL)

sops --encrypt --in-place deploy/base/secrets.sops.yaml
git add deploy/base/secrets.sops.yaml .sops.yaml
```

To edit secrets later:
```bash
sops deploy/base/secrets.sops.yaml   # opens $EDITOR decrypted, saves re-encrypted
```

### Flux wiring

**bjw-s HelmRepository** (if not already in your cluster):
```bash
kubectl apply -f deploy/flux/helmrepository-bjw-s.yaml
```

**Add acurve to your cluster-config repo:**

Copy [`deploy/flux/kustomization-acurve.yaml`](deploy/flux/kustomization-acurve.yaml) into your cluster-config repo (e.g. `clusters/main/apps/acurve.yaml`).

Update `sourceRef.name` if the GitRepository pointing at this repo has a different name than `flux-system`.

### CNPG (if not already installed)

```bash
kubectl apply --server-side \
  -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/main/releases/cnpg-latest.yaml
```

### Switch LLM provider

Edit [`deploy/base/helmrelease.yaml`](deploy/base/helmrelease.yaml) — find the summarizer `env` block and change:

```yaml
LLM_PROVIDER: "ollama"    # or "anthropic"
OLLAMA_URL is read from the acurve-secrets secret
```

### Deploy

```bash
git push

# Force Flux to reconcile immediately
flux reconcile kustomization acurve --with-source

# Watch
flux get helmreleases -n acurve
kubectl get pods -n acurve
kubectl logs -n acurve -l app.kubernetes.io/name=orchestrator -f
```

### First digest

Once pods are running:

```bash
# Trigger a scrape manually (or wait for the 3-hour CronJob)
kubectl create job --from=cronjob/acurve-scraper manual-scrape-1 -n acurve

# Trigger summarizer
kubectl create job --from=cronjob/acurve-summarizer manual-summarize-1 -n acurve

# Preview
kubectl exec -n acurve deploy/acurve-orchestrator -- \
  curl -s http://localhost:8080/api/digest/preview

# Send
kubectl exec -n acurve deploy/acurve-orchestrator -- \
  curl -sX POST http://localhost:8080/api/digest/send
```

---

## Architecture

```
[RSS / YouTube RSS / Reddit JSON]
            │
            ▼  every 3 h
   ┌────────────────────────────────┐
   │  namespace: acurve             │
   │                                │
   │  Scraper CronJob ──► Postgres  │
   │                      (CNPG)   │
   │  Summarizer CronJob ──────────►│ (reads items, writes summaries)
   │   every 30 min                 │
   │                                │
   │  Orchestrator Deployment       │
   │   - REST API /api/*            │
   │   - HTMX dashboard /           │
   │   - Discord webhook adapter    │
   │                                │
   │  Digest CronJob  ──────────────┤ 07:00 + 19:00 Oslo time
   │   (curl + curlimages/curl)     │ POST /api/digest/send
   └────────────────────────────────┘
                   │
                   ▼
            Discord webhook
```

### Services

| Component | Kind | Schedule | Image |
|-----------|------|----------|-------|
| orchestrator | Deployment | always-on | `ghcr.io/konk32/acurve-orchestrator` |
| scraper | CronJob | `0 */3 * * *` | `ghcr.io/konk32/acurve-scraper` |
| summarizer | CronJob | `*/30 * * * *` | `ghcr.io/konk32/acurve-summarizer` |
| digest | CronJob | `0 7,19 * * *` TZ=Europe/Oslo | `curlimages/curl` |

---

## Environment variables reference

### Orchestrator

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DB_URL` | Yes | — | PostgreSQL connection string |
| `DISCORD_WEBHOOK_URL` | No | — | Discord webhook; digest send is a no-op if unset |
| `PORT` | No | `8080` | HTTP listen port |
| `MIGRATIONS_DIR` | No | `../migrations` | Path to goose migration files |

### Scraper

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DB_URL` | Yes | — | PostgreSQL connection string |

### Summarizer

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DB_URL` | Yes | — | PostgreSQL connection string |
| `LLM_PROVIDER` | No | `anthropic` | `anthropic` or `ollama` |
| `ANTHROPIC_API_KEY` | If provider=anthropic | — | Anthropic API key |
| `ANTHROPIC_MODEL` | No | `claude-haiku-4-5-20251001` | Anthropic model ID |
| `OLLAMA_URL` | If provider=ollama | — | Ollama base URL e.g. `http://10.0.0.1:11434` |
| `OLLAMA_MODEL` | No | `gemma3:12b` | Ollama model name |
| `BATCH_SIZE` | No | `20` | Items processed per run |

---

## Digest scoring

Items in the `summaries` table have a `score` (0–100) assigned by the LLM:

| Score | Meaning | Included in digest? |
|-------|---------|---------------------|
| 90–100 | Critical — directly affects your tools | Yes |
| 70–89 | Important — significant releases, CVEs | Yes |
| 50–69 | Interesting but tangential | No (unless fallback triggers) |
| 0–49 | Skip — fluff, unrelated | No |

**Fallback:** if fewer than 3 items qualify at score ≥ 70, the threshold drops to 60 for that run to avoid empty digests.

Items included in a successful digest are recorded in the `digests` table and will not be included in future digests.

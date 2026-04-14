FROM golang:1.23-alpine AS builder

WORKDIR /build

# Cache deps
COPY orchestrator/go.mod orchestrator/go.sum ./
RUN go mod download

# Build
COPY orchestrator/ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /acurve ./cmd/acurve

# ---
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /acurve /acurve
COPY migrations/ /migrations

ENV MIGRATIONS_DIR=/migrations

EXPOSE 8080
ENTRYPOINT ["/acurve"]

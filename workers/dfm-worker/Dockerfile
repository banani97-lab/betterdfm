# NOTE: This Dockerfile must be built from the monorepo root because of the
# go.mod replace directive pointing to ../../engine/dfm-engine.
# Build command (run from /betterdfm repo root):
#   docker build -f workers/dfm-worker/Dockerfile -t dfm-worker .

FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git
WORKDIR /workspace
# Copy engine module first (needed for replace directive)
COPY engine/dfm-engine /workspace/engine/dfm-engine
# Copy worker
WORKDIR /workspace/workers/dfm-worker
COPY workers/dfm-worker/go.mod ./
COPY workers/dfm-worker/ .
RUN go mod tidy
RUN go build -o /worker ./cmd/worker

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /worker /worker
CMD ["/worker"]

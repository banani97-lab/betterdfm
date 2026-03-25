# NOTE: This Dockerfile must be built from the monorepo root because of the
# go.mod replace directive pointing to ../../engine/dfm-engine.
# Build command (run from /betterdfm repo root):
#   docker build -f apps/api/Dockerfile -t betterdfm-api .

FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git
WORKDIR /workspace
# Copy engine module first (needed for replace directive)
COPY engine/dfm-engine /workspace/engine/dfm-engine
# Copy API
WORKDIR /workspace/apps/api
COPY apps/api/go.mod ./
COPY apps/api/ .
RUN go mod tidy
RUN go build -o /server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /server /server
EXPOSE 8080
CMD ["/server"]

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
# BUILD_TAGS is empty for production images (no dev auth bypass). Local dev
# (docker-compose) passes BUILD_TAGS=dev to keep the JWT_ISSUER-empty bypass.
ARG BUILD_TAGS=""
RUN go build -tags "$BUILD_TAGS" -o /server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /server /server
EXPOSE 8080
CMD ["/server"]

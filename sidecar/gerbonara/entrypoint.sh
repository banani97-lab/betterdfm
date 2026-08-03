#!/bin/sh
set -e
# Internal service-to-service TLS (NIST 800-171 3.13.8). When TLS_CERT/TLS_KEY
# are injected (from Secrets Manager, GovCloud), serve HTTPS so worker->gerbonara
# traffic is encrypted in transit. Falls back to HTTP when they are absent
# (local dev), so the dev workflow is unchanged.
if [ -n "${TLS_CERT:-}" ] && [ -n "${TLS_KEY:-}" ]; then
  printf '%s' "$TLS_CERT" > /tmp/tls.crt
  printf '%s' "$TLS_KEY" > /tmp/tls.key
  chmod 600 /tmp/tls.key
  echo "gerbonara: serving HTTPS (internal TLS)"
  exec uvicorn main:app --host 0.0.0.0 --port 8001 --no-access-log \
    --ssl-certfile /tmp/tls.crt --ssl-keyfile /tmp/tls.key
fi
echo "gerbonara: serving HTTP (no TLS cert provided)"
exec uvicorn main:app --host 0.0.0.0 --port 8001 --no-access-log

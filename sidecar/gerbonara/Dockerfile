# RapidDFM gerbonara sidecar
FROM python:3.11-slim

WORKDIR /app

# Install dependencies
COPY pyproject.toml ./
RUN pip install --no-cache-dir fastapi "uvicorn[standard]" boto3 "pydantic>=2.7.0"

COPY main.py models.py units.py parser_odb.py storage.py entrypoint.sh ./
RUN chmod +x entrypoint.sh

EXPOSE 8001
# entrypoint.sh serves HTTPS when TLS_CERT/TLS_KEY are injected (GovCloud
# internal TLS), else HTTP (dev). --no-access-log drops the per-request stdout
# line (ALB/health hits were a large share of the CloudWatch ingestion bill).
CMD ["./entrypoint.sh"]

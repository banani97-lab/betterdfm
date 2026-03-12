FROM python:3.11-slim

WORKDIR /app

# Install dependencies
COPY pyproject.toml ./
RUN pip install --no-cache-dir fastapi "uvicorn[standard]" gerbonara boto3 "pydantic>=2.7.0"

COPY main.py models.py units.py parser_odb.py parser_gerber.py storage.py ./

EXPOSE 8001
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8001"]

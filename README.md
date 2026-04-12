# RapidDFM

A SaaS PCB Design-for-Manufacturability (DFM) analysis engine. Contract manufacturers define their shop's capability profile; their customers upload ODB++ files and receive instant, visual, actionable feedback.

## Core Concepts

### What is DFM Analysis?

Design-for-Manufacturability (DFM) is the practice of checking a PCB design against a fabrication shop's physical constraints before sending it to production. Violations — traces too narrow, drills too small, copper too close to the board edge — caught early save time and money. RapidDFM automates this check.

### Who Uses It?

- **Contract Manufacturers (CMs)** — subscribe, configure their capability profile, and white-label the portal for their customers.
- **PCB Designers** — upload files, get results in seconds, fix issues before tape-out.

### Capability Profiles

Each CM defines one or more **Capability Profiles** that encode their shop-floor constraints:

| Constraint | Description |
|---|---|
| `minTraceWidthMM` | Narrowest trace the fab can reliably etch |
| `minClearanceMM` | Minimum gap between copper features |
| `minDrillDiamMM` | Smallest drill the shop can run |
| `maxDrillDiamMM` | Largest drill before routing is required |
| `minAnnularRingMM` | Minimum copper ring around a drilled hole |
| `maxAspectRatio` | Board thickness / drill diameter limit |
| `minSolderMaskDamMM` | Minimum solder mask sliver between pads |
| `minEdgeClearanceMM` | Keep-out margin from board outline |

### DFM Rules (MVP)

| Rule | Severity | What It Checks |
|---|---|---|
| `trace-width` | ERROR | Any trace narrower than `minTraceWidthMM` |
| `clearance` | ERROR | Trace-to-trace or trace-to-pad gap below `minClearanceMM` |
| `drill-size` | ERROR | Drill diameter out of `[minDrillDiamMM, maxDrillDiamMM]` |
| `annular-ring` | ERROR | Copper ring around via/drill below `minAnnularRingMM` |
| `aspect-ratio` | WARNING | Board thickness / drill diameter exceeds `maxAspectRatio` |
| `solder-mask-dam` | WARNING | Solder mask bridge between pads below `minSolderMaskDamMM` |
| `edge-clearance` | WARNING | Copper within `minEdgeClearanceMM` of board outline |

---

## Architecture

```
Browser
  ├── POST /submissions       → create record + get presigned S3 URL
  ├── PUT  <presigned URL>    → upload file directly to S3
  ├── POST /submissions/:id/analyze  → enqueue SQS job
  ├── GET  /jobs/:id          → poll until DONE
  └── GET  /jobs/:id/violations + board → render results

ECS Fargate Worker
  ← SQS { jobId }
  → fetch file from S3
  → POST gerbonara sidecar /parse   → BoardData JSON
  → run DFM engine rules against BoardData + CapabilityProfile
  → write Violations to PostgreSQL
  → mark job DONE
```

### Services

| Service | Tech | Port | Purpose |
|---|---|---|---|
| `web` | Next.js 14 (TypeScript) | 3000 | Customer-facing portal |
| `api` | Go + Echo | 8080 | REST API, auth, job dispatch |
| `worker` | Go (SQS poller) | — | Async DFM analysis runner |
| `gerbonara` | Python FastAPI | 8001 | ODB++ file parser |
| `postgres` | PostgreSQL 16 | 5432 | Primary database |

### Monorepo Layout

```
rapiddfm/
├── apps/
│   ├── web/                  # Next.js 14 frontend
│   └── api/                  # Go Echo API server
├── engine/
│   └── dfm-engine/           # Go DFM rule library (shared module)
├── workers/
│   └── dfm-worker/           # Go SQS worker (runs on ECS Fargate)
├── sidecar/
│   └── gerbonara/            # Python FastAPI + gerbonara parser
├── docker-compose.yml
└── .env.example
```

---

## Running Locally

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- (Optional, for individual service dev) Go 1.22+, Node.js 20+, pnpm, Python 3.11+

### Quick Start

```bash
# 1. Clone and enter the repo
git clone https://github.com/your-org/rapiddfm.git
cd rapiddfm

# 2. Copy the environment file
cp .env.example .env

# 3. Start all services
docker-compose up --build
```

| URL | Service |
|---|---|
| http://localhost:3000 | Web frontend |
| http://localhost:8080 | Go API |
| http://localhost:8001 | Gerbonara sidecar |
| localhost:5432 | PostgreSQL |

> **Auth bypass**: Leave `JWT_ISSUER` empty in `.env` (the default). All API requests will be treated as an authenticated admin belonging to the `default-org` organization — no Cognito setup required for local development.

> **AWS bypass**: Leave `AWS_ACCESS_KEY_ID` and `SQS_QUEUE_URL` empty. The API will issue fake presigned URLs, the worker will skip SQS polling, and the sidecar will return mock board data. The full analysis flow runs end-to-end without any AWS account.

### Environment Variables

| Variable | Required in Prod | Description |
|---|---|---|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `AWS_REGION` | Yes | AWS region (e.g. `us-east-1`) |
| `AWS_ACCESS_KEY_ID` | Yes | AWS credentials |
| `AWS_SECRET_ACCESS_KEY` | Yes | AWS credentials |
| `S3_BUCKET` | Yes | S3 bucket for file uploads |
| `SQS_QUEUE_URL` | Yes | SQS queue URL for job dispatch |
| `JWT_ISSUER` | Yes | Cognito issuer URL — leave empty to disable auth in dev |
| `COGNITO_USER_POOL_ID` | Yes | Cognito User Pool ID |
| `COGNITO_CLIENT_ID` | Yes | Cognito App Client ID |
| `COGNITO_DOMAIN` | Yes | Cognito hosted UI domain |
| `GERBONARA_URL` | Yes | Internal URL of the parser sidecar |
| `NEXT_PUBLIC_API_URL` | Yes | Browser-visible API base URL |

### Running Services Individually

```bash
# Frontend (Next.js)
pnpm dev:web

# API (Go)
pnpm dev:api

# Gerbonara sidecar (Python)
pnpm dev:sidecar
```

---

## API Reference

```
POST   /auth/callback              Cognito OIDC callback
GET    /auth/me                    Current user info

GET    /submissions                List submissions
POST   /submissions                Create submission → returns presigned S3 PUT URL
GET    /submissions/:id            Get submission detail
POST   /submissions/:id/analyze    Start analysis job

GET    /jobs/:id                   Poll job status (PENDING | PROCESSING | DONE | FAILED)
GET    /jobs/:id/violations        List violations for a completed job
GET    /jobs/:id/board             Parsed board geometry (for the viewer)
GET    /jobs/:id/report.pdf        PDF report export

GET    /profiles                   List capability profiles
POST   /profiles                   Create capability profile
GET    /profiles/:id               Get profile
PUT    /profiles/:id               Update profile
DELETE /profiles/:id               Delete profile
```

---

## Deployment (AWS)

| Resource | Service |
|---|---|
| API + Worker | ECS Fargate |
| Database | RDS PostgreSQL |
| File storage | S3 |
| Job queue | SQS |
| Auth | Cognito User Pools |
| Frontend | Vercel or CloudFront + S3 |

The worker Dockerfile must be built from the **monorepo root** because it imports the `engine/dfm-engine` Go module via a `replace` directive:

```bash
docker build -f workers/dfm-worker/Dockerfile .
```

---

## License

See [LICENSE](LICENSE).

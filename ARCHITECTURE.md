# RapidDFM — Architecture

## Overview

RapidDFM is a SaaS PCB Design-for-Manufacturability (DFM) engine. Contract manufacturers (CMs) subscribe, define their shop's capability profile, and offer their customers a self-serve DFM analysis portal.

## Tech Stack

| Concern | Technology |
|---|---|
| Frontend | Next.js 14 (TypeScript, App Router, Tailwind, shadcn/ui) |
| Backend API | Go (Echo framework) |
| DFM Worker | Go (ECS Fargate, polls SQS) |
| DFM Rule Engine | Go library (shared between API and worker) |
| Gerber/ODB++ Parsing | Python FastAPI sidecar wrapping gerbonara |
| Database | AWS RDS PostgreSQL via GORM |
| File Storage | AWS S3 (presigned upload URLs) |
| Job Queue | AWS SQS |
| Auth | AWS Cognito (user pools, JWT OIDC tokens) |
| Results Viewer | SVG board render (@tracespace/render) + violation overlay |

## Monorepo Structure

```
rapiddfm/
├── apps/
│   ├── web/                        # Next.js 14 TypeScript frontend
│   └── api/                        # Go Echo API server
├── workers/
│   └── dfm-worker/                 # Go ECS Fargate SQS worker
├── engine/
│   └── dfm-engine/                 # Go DFM rule library (go module)
├── sidecar/
│   └── gerbonara/                  # Python FastAPI + gerbonara parser
├── infra/                          # AWS CDK TypeScript (future)
├── docker-compose.yml
├── pnpm-workspace.yaml
├── package.json
├── .env.example
├── .gitignore
└── ARCHITECTURE.md
```

## Data Flow

```
Browser
  ├── (1) POST /api/submissions (filename, fileType)
  │         ← { submissionId, presignedUrl }
  ├── (2) PUT presignedUrl (file bytes direct to S3)
  ├── (3) POST /api/submissions/:id/analyze (profileId)
  │         → creates AnalysisJob, enqueues SQS message
  ├── (4) GET /api/jobs/:id  (poll until DONE)
  ├── (5) GET /api/jobs/:id/violations
  └── renders SVG board + overlay violations

ECS Fargate Worker
  ← SQS message { jobId }
  → update job status = PROCESSING
  → fetch file from S3
  → POST http://gerbonara-sidecar/parse  { fileKey, fileType }
        ← BoardData JSON { layers, traces, vias, pads, drills, outline }
  → run DFM rules (engine lib) against BoardData + CapabilityProfile
  → bulk insert Violations into RDS
  → update job status = DONE
```

## Database Schema

See `apps/api/src/db/models.go` for GORM model definitions.

### Core Tables
- **organizations** — CM tenants
- **users** — linked to Cognito sub, belong to an org
- **capability_profiles** — CM shop floor constraints (JSON rules blob)
- **submissions** — uploaded Gerber/ODB++ files
- **analysis_jobs** — one analysis run per submission+profile pair
- **violations** — individual DFM issues found per job

## API Routes

```
POST   /auth/callback              Cognito OIDC callback → issue session
GET    /auth/me                    Current user info

GET    /submissions                List submissions for org
POST   /submissions                Create submission + return presigned S3 URL
GET    /submissions/:id            Get submission detail
POST   /submissions/:id/analyze    Start analysis job (enqueue SQS)

GET    /jobs/:id                   Poll job status
GET    /jobs/:id/violations        Get violation list for completed job

GET    /profiles                   List capability profiles for org
POST   /profiles                   Create capability profile
PUT    /profiles/:id               Update profile
DELETE /profiles/:id               Delete profile
GET    /profiles/:id               Get profile detail
```

## DFM Rules (MVP)

| Rule ID | Check | Severity |
|---|---|---|
| `trace-width` | Any trace narrower than `minTraceWidthMM` | ERROR |
| `clearance` | Trace-to-trace or trace-to-pad gap below `minClearanceMM` | ERROR |
| `drill-size` | Drill diameter below `minDrillDiamMM` or above `maxDrillDiamMM` | ERROR |
| `annular-ring` | Copper ring around via/drill below `minAnnularRingMM` | ERROR |
| `aspect-ratio` | Board thickness / drill diameter exceeds `maxAspectRatio` | WARNING |
| `solder-mask-dam` | Solder mask bridge between pads below `minSolderMaskDamMM` | WARNING |
| `edge-clearance` | Component/copper within `minEdgeClearanceMM` of board outline | WARNING |

## Local Development

```bash
# Copy env file
cp .env.example .env

# Start all services
docker-compose up --build

# Or run individually:
pnpm dev:web        # Next.js on :3000
pnpm dev:api        # Go API on :8080
pnpm dev:sidecar    # Python sidecar on :8001
```

## Deployment (AWS)

- **API + Worker**: ECS Fargate
- **Database**: RDS PostgreSQL
- **Storage**: S3
- **Queue**: SQS
- **Auth**: Cognito User Pools
- **Frontend**: Vercel or CloudFront + S3

# RapidDFM

Design-for-manufacturability (DFM) analysis platform for PCB designs. Users upload Gerber or ODB++ files, the system parses board geometry, runs 11 manufacturing rule checks against a capability profile, and presents violations with a visual board viewer.

## Architecture

Four services in a monorepo, orchestrated via Docker Compose:

| Service | Stack | Port | Location |
|---------|-------|------|----------|
| **web** | Next.js 14 (App Router), TypeScript, Tailwind CSS | 3000 | `apps/web/` |
| **api** | Go 1.23, Echo v4, GORM, PostgreSQL 16 | 8080 | `apps/api/` |
| **worker** | Go 1.23, SQS consumer | - | `workers/dfm-worker/` |
| **gerbonara** | Python 3.11+, FastAPI, gerbonara lib | 8001 | `sidecar/gerbonara/` |

**DFM Engine** (shared Go module): `engine/dfm-engine/` — imported by both api and worker via local `replace` directive.

### Data flow

```
User uploads file
  → web: createSubmission() gets presigned S3 URL
  → browser uploads directly to S3
  → web: startAnalysis() calls API
  → api: creates AnalysisJob (PENDING), enqueues SQS message
  → worker: polls SQS, calls gerbonara POST /parse
  → gerbonara: downloads from S3, parses Gerber/ODB++, returns BoardData
  → worker: runs 11 DFM rules, computes score, stores violations + board data
  → web: polls GET /jobs/:id until DONE, renders results
```

**The worker is the only service that calls gerbonara.** The API never calls gerbonara directly.

### Database

PostgreSQL 16 with GORM auto-migration. 6 tables:
- `organizations` — multi-tenant (contract manufacturers)
- `users` — linked to AWS Cognito, roles: ADMIN | ANALYST | VIEWER
- `capability_profiles` — manufacturing rules as JSONB (11 parameters)
- `submissions` — uploaded file metadata, status: UPLOADED → ANALYZING → DONE | FAILED
- `analysis_jobs` — job runs with board_data (JSONB), mfg_score, mfg_grade
- `violations` — individual DFM issues with X/Y coordinates, severity, measurements

### Auth

AWS Cognito (OIDC + JWT RS256). **Dev mode**: when `JWT_ISSUER` (backend) or `NEXT_PUBLIC_COGNITO_CLIENT_ID` (frontend) is empty, auth is bypassed with a dev user/token.

## Development

### Running locally

```bash
# All services via Docker
pnpm docker:up

# Individual services (run each in a separate terminal)
pnpm dev:web        # Next.js on :3000
pnpm dev:api        # Go API on :8080
pnpm dev:worker     # Go worker (polls DB in dev mode when SQS_QUEUE_URL is empty)
pnpm dev:sidecar    # Python FastAPI on :8001
```

PostgreSQL must be running. Use `docker-compose up postgres` if running services individually.

### Testing

```bash
# Frontend (Vitest + Testing Library)
cd apps/web && pnpm test          # watch mode
cd apps/web && pnpm test:run      # single run

# DFM engine (Go)
cd engine/dfm-engine && go test ./...

# Sidecar (pytest)
cd sidecar/gerbonara && pytest

# Worker (Go)
cd workers/dfm-worker && go test ./...
```

### CI

GitHub Actions (`.github/workflows/ci.yml`): runs engine tests, worker build, api build, sidecar tests, and frontend tests on push to main or PR.

Deploy (`.github/workflows/deploy.yml`): path-filtered — only rebuilds/deploys services with changes. Worker + gerbonara → ECS. API → App Runner. Web → Vercel.

## Key conventions

### Go (API + Worker + Engine)

- **Handler pattern**: struct with `db` and `aws` fields, constructor `NewXHandler()`, methods are Echo handlers.
- **Error handling**: `echo.NewHTTPError(status, message)` for HTTP errors. `errors.Is(err, gorm.ErrRecordNotFound)` for 404s.
- **No API-layer tests** — test coverage is focused on the DFM engine rules.
- Models defined in `apps/api/src/db/models.go`. Worker mirrors them in `workers/dfm-worker/internal/models.go`.

### TypeScript (Frontend)

- **No state management library** — React hooks + localStorage only.
- **API client**: `apiFetch<T>(path, init)` in `src/lib/api.ts`. All types co-located there.
- **Styling**: Tailwind CSS with CSS variables (HSL). `cn()` utility (clsx + tailwind-merge). CVA for button variants.
- **Canvas rendering**: separated into pure `boardPainter.ts` (testable) and impure `canvasRenderer.ts`.
- **Largest component**: `BoardViewer.tsx` (~1165 lines) — handles canvas visualization and interaction.

### Python (Sidecar)

- **Two parsers**: `parser_gerber.py` (uses gerbonara LayerStack) and `parser_odb.py` (custom archive extraction).
- **All coordinates output in millimeters** — unit conversion happens in parsers.
- **Fallback mock data** if S3 is unavailable (dev mode).

## DFM rules

11 rules in `engine/dfm-engine/rule_*.go`, each implementing the `Rule` interface (`ID() string`, `Run(board, profile) []Violation`):

| Rule | Severity | What it checks |
|------|----------|---------------|
| trace-width | ERROR | Trace width >= minTraceWidthMM |
| clearance | ERROR | Trace/pad spacing >= minClearanceMM |
| drill-size | ERROR | Drill diameter within min/max bounds |
| annular-ring | ERROR | Copper ring around vias >= minAnnularRingMM |
| drill-to-drill | ERROR | Hole-to-hole spacing >= minDrillToDrillMM |
| drill-to-copper | ERROR | Hole-to-trace clearance >= minDrillToCopperMM |
| aspect-ratio | WARNING | Board thickness / drill diameter <= maxAspectRatio |
| solder-mask-dam | WARNING | Solder mask bridge between pads >= minSolderMaskDamMM |
| edge-clearance | WARNING | Copper distance from board outline >= minEdgeClearanceMM |
| copper-sliver | WARNING | Copper feature width >= minCopperSliverMM |
| silkscreen-on-pad | INFO | Silkscreen does not overlap pads |

**Scoring** (`score.go`): weighted penalties by rule (clearance=3.0 is heaviest) and severity (ERROR=10x, WARNING=3x, INFO=0.5x). Score 0-100, grades A/B/C/D.

## Environment variables

See `.env.example` for all variables. Key dev-mode behaviors:
- Empty `JWT_ISSUER` → auth bypassed (Go API)
- Empty `NEXT_PUBLIC_COGNITO_CLIENT_ID` → auth bypassed (frontend)
- Empty `SQS_QUEUE_URL` → worker polls DB instead of SQS
- Empty AWS credentials → gerbonara returns mock board data

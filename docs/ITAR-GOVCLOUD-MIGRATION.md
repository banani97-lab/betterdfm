# ITAR / AWS GovCloud Migration Plan

**Status:** Planning
**Scope:** Two layers. (1) The engineering work to make ITAR / CUI-controlled PCB design data legally storable and processable inside a US-person GovCloud boundary. (2) The path to **FedRAMP Moderate equivalency**, which the go-to-market (selling to defense-supply-chain CMs) makes mandatory, not optional.
**Last updated:** 2026-06-04

---

## 1. Framing

Three things get conflated and should be kept separate:

- **ITAR** (22 CFR 120-130) is an access-control and data-residency regime, not a cloud certification. The core rule: ITAR technical data (customer PCB designs for defense electronics, likely USML Cat XI) may only be accessed by **US persons** and must not be exported, including "deemed exports" to any foreign person who touches the data (an admin or support engineer counts).
- **AWS GovCloud (US)** is the mechanism: US-soil regions operated by screened US persons. It is how residency + personnel requirements are met. GovCloud is itself FedRAMP High authorized, so building on it lets us inherit a large pool of controls.
- **NIST SP 800-53 / FedRAMP** is the control catalog GovCloud is assessed against, and the bar that flows down to **us as a cloud provider** (see section 2).

**Key architectural lever:** the 2020 DDTC encryption carve-out (**22 CFR 120.54(a)(5)**) means properly end-to-end-encrypted technical data, with keys held only by US persons and never decrypted abroad, is not an export. This makes a customer-managed KMS CMK with a US-person-restricted key policy a load-bearing design decision.

---

## 2. Why the compliance bar is FedRAMP Moderate equivalency

RapidDFM is being marketed to **Contract Manufacturers (CMs)** who subcontract to **OEMs with defense contracts**. Walk the chain:

1. The OEM's DoD contract carries **DFARS 252.204-7012** and increasingly **252.204-7021 (CMMC)**.
2. Those clauses flow down to the CM.
3. The CM uses RapidDFM to store and process the OEM's board design data, which is **CUI** (covered defense information) and likely **ITAR** technical data.
4. RapidDFM is therefore an **external cloud service provider handling CUI** on behalf of defense contractors.

The binding requirement is **DFARS 252.204-7012(b)(2)(ii)(D)**: a CSP that stores, processes, or transmits CUI must meet the **FedRAMP Moderate baseline (or equivalent)** and comply with the incident-reporting clauses (c)-(g). Customers are contractually obligated to verify this before putting CUI in our product. **CMMC 2.0** reinforces it: the CM needs **CMMC Level 2 / NIST 800-171 (110 controls)**, and as their CUI-handling CSP we land inside their assessment scope.

**Asymmetry to internalize:** the CM only needs the 110-control 800-171 bar. We, as the cloud provider holding their CUI, need the ~325-control FedRAMP Moderate bar, roughly 3x the burden of the customers we sell to.

### Equivalency vs full authorization

We target **equivalency**, not full authorization:

- **FedRAMP Moderate equivalency:** meet 100% of FedRAMP Moderate controls, have a **3PAO** assess the body of evidence (SSP, SAR, POA&M), and hand that package to each customer. Per DoD's Dec 2023 equivalency memo this is strict: 100% met, no "in process," 3PAO-assessed. It skips the government authorization and marketplace listing but **not** the 3PAO assessment.
- **Full FedRAMP Moderate authorization:** adds a government sponsor (agency ATO) or a streamlined pathway, PMO review, marketplace listing, and reusability across customers. Heavier and slower; revisit later once volume justifies it.

A 3PAO assessment is therefore **in scope**, just not a full FedRAMP PMO authorization.

---

## 3. Deployment model decision: multi-tenant SaaS

**Decision: multi-tenant SaaS, RapidDFM as CSP of record.** We carry the FedRAMP Moderate equivalency burden.

**Why not the single-tenant "customer enclave" model** (deploy into each customer's own GovCloud boundary and inherit their authorization): it only works if the customer already has a CMMC-assessed enclave to ride. Our alpha and core market is **small subcontractors**, the cohort least likely to have one, and per-customer enclave economics don't survive small-sub budgets. Both the market structure and the customer profile point to multi-tenant.

**Consequence: we become an aggregation target.** One boundary holding many defense subcontractors' CUI is a high-value target, and assessors scrutinize **tenant isolation** hard. This promotes tenant-isolation hardening to a first-class workstream (WS7).

---

## 4. Alpha strategy: validate product without touching CUI

We cannot legally hold a customer's real CUI until our boundary is assessed, but we don't want to spend on the assessment before product-market fit. Resolve the chicken-and-egg by **decoupling product validation from CUI handling**:

1. **Run the alpha on non-CUI / synthetic / representative boards.** DFM value is demonstrable on realistic-but-uncontrolled designs; no real ITAR geometry is needed to validate the rules engine and UX. No compliance gate blocks the alpha.
2. **Build the GovCloud foundation in parallel** (WS0-WS6, needed regardless).
3. **Pursue equivalency once the alpha shows PMF**, ideally with a design partner willing to co-commit, so the 3PAO spend is sequenced behind evidence.
4. **Flip on real-CUI ingestion only after the 3PAO assessment lands.** Until then, enforce a **hard product guardrail** that controlled data cannot be uploaded (terms + a technical gate). Small subs are sometimes loose about marking CUI, so this must be enforced, not merely requested.

---

## 5. Target architecture

```
GovCloud (partition aws-us-gov, region us-gov-west-1)
Single VPC, private subnets

  CloudFront(gov) + ALB ──► web (ECS/Fargate, Next.js)   ◄── was Vercel
                            │
                            └─► API (ECS/Fargate, Go)     ◄── was App Runner
                                   │
                     ┌─────────────┼───────────────┐
              RDS Postgres     SQS (gov)      S3 (gov, SSE-KMS)
              (encrypted,          │           per-tenant prefixes,
               tenant-isolated)    │           TLS-only
                                worker (Fargate) ──TLS──► gerbonara (Fargate)

  Cognito (gov, MFA)   KMS CMK (US-person key policy)   CloudTrail + central logs
```

Everything that currently lives on Vercel or App Runner, or that talks to OpenAI, moves inside this boundary.

---

## 6. Current-state blockers (grounded in the repo)

Four things are outright incompatible with the boundary:

| # | Blocker | Evidence |
|---|---------|----------|
| 1 | **Web app hosted on Vercel** (not GovCloud, not FedRAMP, globally distributed) | `vercel.json`, `.github/workflows/deploy.yml` |
| 2 | **API runs on AWS App Runner**, which does not exist in GovCloud | `.github/workflows/deploy.yml:12` (App Runner ARN) |
| 3 | **OpenAI egress** sends derived DFM technical data to a non-US-person service | `apps/web/src/app/api/ai/submission-overview/route.ts:257` |
| 4 | **Hardcoded to commercial `us-east-1`** (different AWS partition than gov) | `.github/workflows/deploy.yml:8-12`, `.env.example`, ECR registry `578381014577.dkr...` |

Plus two high-severity application gaps:

- **Dev-mode auth bypass.** `apps/api/src/lib/auth.go:169` and `:206` skip all auth when `JWT_ISSUER` is empty, defaulting to an ADMIN dev user. Same on the frontend when `NEXT_PUBLIC_COGNITO_CLIENT_ID` is empty. In a CUI system this path must be impossible to reach in production, not merely unconfigured.
- **Tenant isolation is commercial-grade.** Multi-tenancy is an `organizations` column over shared Postgres / shared S3 bucket / shared SQS. Adequate for commercial SaaS, but with multi-tenant CUI it is a primary control surface (see WS7).

---

## 7. Control-level gaps (mapped to 800-53 / FedRAMP Moderate families)

| Area | Current state | Needed |
|------|---------------|--------|
| Data residency (PE/SC) | Commercial us-east-1, Vercel global | GovCloud only; FIPS 140-2/3 validated endpoints |
| Encryption in transit (SC-8) | worker -> gerbonara over plain `http://` (`docker-compose.yml:49`) | TLS 1.2+ everywhere incl. service-to-service; FIPS endpoints |
| Encryption at rest (SC-28) | Not enforced | SSE-KMS on S3, encrypted RDS/EBS, customer-managed CMK (enables the 120.54 position); consider per-tenant data keys |
| Tenant isolation (AC-3/AC-4/SC-4) | `organizations` column, shared stores | Enforced per-tenant isolation, proven by authz tests; per-tenant S3 prefixes/keys (see WS7) |
| Identity / MFA (IA-2) | Cognito; dev bypass when issuer empty | MFA mandatory; no dev-bypass path in prod; US-person identity proofing |
| Credentials (IA-5/AC-6) | Static `AWS_ACCESS_KEY_ID`/`SECRET` in env + GH secrets (`deploy.yml:48-52`) | IAM roles / OIDC, no long-lived keys, least privilege |
| Audit logging (AU family) | None evident | CloudTrail (mgmt + S3 data events), centralized immutable logs, retention, review; CUI access attributable to tenant + US person |
| Cyber incident reporting (IR) | None | DFARS 7012 (c)-(g): 72-hour reporting capability, DIBNet process |
| CI/CD supply chain (CM/SA) | GitHub-hosted runners (potentially non-US), static deploy keys | US-person-controlled pipeline; GovCloud CodePipeline or US self-hosted runners; artifact integrity |
| Network (SC-7) | Public services | VPC, private subnets, ALB-only ingress, flow logs, no public DB |
| Personnel (PS/AC) | n/a | US-persons-only operational access (HR/process control) |
| Boundary docs (CA/PL) | n/a | SSP, SAR (3PAO), POA&M, incident response plan: the equivalency body of evidence |

---

## 8. Workstreams

### WS0: Boundary + account (gates everything, non-code)
- Confirm DDTC registration; define the authorization boundary on paper (which services, which data).
- Stand up the GovCloud account, US-person-only IAM, VPC with private subnets.
- Create the KMS CMK whose key policy is restricted to US-person principals. Underpins the 22 CFR 120.54 encryption position.

### WS1: Partition / region parameterization (code, low risk, do first)
SDK clients use `LoadDefaultConfig` with no explicit endpoint, so partition resolves from region. Make region/partition config-driven and force FIPS endpoints.
- `apps/api/src/lib/aws.go:29` and `workers/dfm-worker/cmd/worker/main.go:35`: add `config.WithUseFIPSEndpoint(aws.FIPSEndpointStateEnabled)`. Region from `AWS_REGION=us-gov-west-1`.
- `sidecar/gerbonara/storage.py:22`: boto3 client defaults region to `us-east-1` and uses no FIPS. Switch to `Config(use_fips_endpoint=True)` and drop the `us-east-1` default so a missing region fails loud.
- `.github/workflows/deploy.yml:7-12`: every hardcoded value (region, ECR account, App Runner ARN) becomes a GovCloud value / GitHub env var. Partition prefix becomes `arn:aws-us-gov:...`.
- `.env.example`: regions, Cognito issuer URL (`cognito-idp.us-gov-west-1.amazonaws.com/...`), SQS URL, S3 bucket move to gov.

### WS2: Re-platform the two non-GovCloud services
- **Web off Vercel.** Delete `vercel.json`. The web app has `apps/web/Dockerfile`, so it becomes an ECS/Fargate service behind ALB/CloudFront. It has server-side API routes, so it must run as a Node server, not a static export.
- **API off App Runner.** Replace `aws apprunner start-deployment` in `deploy.yml:133-176` with the ECS register-task-definition + update-service pattern the worker already uses (`deploy.yml:77-131`). Fold the API into the existing `betterdfm` ECS cluster.

### WS3: Remove OpenAI egress (decided: kill it, no replacement)
The only outbound call to a non-US-person service. It powers one cosmetic feature: the plain-English "submission overview" blurb. A complete deterministic fallback already exists and runs whenever the key is absent, so no feature visibly disappears.
- `apps/web/src/app/api/ai/submission-overview/route.ts`: delete the two `OPENAI_*` constants, the `generateOverviewWithAI` function, and the `if (OPENAI_API_KEY)` upgrade block. `fallbackOverview` becomes the sole path. Response shape (`overview`, `counts`, `generatedWith`) is unchanged, so the results page needs no edit.
- `.env.example`: drop `OPENAI_API_KEY` and `OPENAI_OVERVIEW_MODEL`.
- No Go, worker, sidecar, or docker-compose changes. Zero GovCloud dependency, can land in the current repo any time.

### WS4: Encryption (at rest + the internal hop)
- **S3:** enforce SSE-KMS with the WS0 CMK; bucket policy denying non-TLS (`aws:SecureTransport`) and unencrypted puts.
- **RDS Postgres:** encrypted storage with the CMK, private subnet, TLS required (`rds.force_ssl`), `DATABASE_URL` with `sslmode=require`.
- **Internal TLS:** worker/API reach gerbonara over plain `http://` (`docker-compose.yml:49`). Lower priority inside a private subnet, but SC-8 wants it encrypted; terminate TLS at gerbonara or front it with an internal ALB.

### WS5: Identity hardening
- **Kill the dev auth bypass.** Move the bypass in `apps/api/src/lib/auth.go:169` and `:206` behind a `//go:build dev` build tag so the production binary cannot contain it; make the prod entrypoint hard-fail if `JWT_ISSUER` is empty. Same for the frontend `NEXT_PUBLIC_COGNITO_CLIENT_ID` bypass. The JWT validation itself (`auth.go:109`) is sound.
- **MFA mandatory** on the gov Cognito pool; US-person identity proofing at enrollment.
- **No long-lived keys.** `deploy.yml:48-52` uses static `AWS_ACCESS_KEY_ID`/`SECRET`. Replace with GitHub OIDC -> IAM role assumption. Fargate tasks get IAM task roles. Static keys in `.env.example` / `docker-compose.yml` are dev-only and must never exist in gov.

### WS6: Audit + incident reporting baseline
- CloudTrail on (management + S3 data events for the uploads bucket).
- Centralized CloudWatch logs with retention; VPC flow logs.
- Every CUI access attributable to a tenant + a US person.
- DFARS 7012 (c)-(g): a 72-hour cyber-incident reporting capability and DIBNet process.
- Access-review process for the US-person list.

### WS7: Tenant isolation hardening (new, first-class for multi-tenant CUI)
Multi-tenant CUI makes us an aggregation target; isolation must be enforced and provable.
- Enforce per-tenant data isolation: row-level security or per-tenant schemas in Postgres; per-tenant S3 prefixes or buckets with scoped key policies.
- Authz tests proving one org can never read another's submissions, violations, or board data.
- Consider per-tenant KMS data keys so a key-policy mistake cannot cross tenants.
- Monitoring that attributes every CUI access to a tenant + US person (ties to WS6).

---

## 9. Sequencing

| Phase | Work | Type | Gates on |
|-------|------|------|----------|
| 1 | Product alpha on **non-CUI / synthetic** boards + hard CUI-upload guardrail | product/code | none |
| 2 | WS1 region+FIPS parameterization, WS3 OpenAI removal | code (small) | none, safe in current repo |
| 3 | WS0 account / boundary / KMS | infra/ops | prerequisite for 4-6 |
| 4 | WS5 dev-bypass build tags + IAM/OIDC, WS7 tenant isolation | code | WS0 for IAM/OIDC |
| 5 | WS2 re-platform web + API onto Fargate | infra + CI | WS0 |
| 6 | WS4 encryption, WS6 audit + incident reporting | infra | WS0 |
| 7 | Engage 3PAO; assemble SSP/SAR/POA&M; **FedRAMP Moderate equivalency** | assessment | PMF signal + WS0-WS7 |
| 8 | Flip on real-CUI ingestion | product | equivalency complete |

Phases 1-2 can start in the current repo today and carry no GovCloud dependency. Real CUI is gated until phase 8.

---

## 10. Open decisions

1. **CI/CD personnel boundary.** GitHub-hosted runners can run on non-US infrastructure. Building images (code, not CUI) is lower risk, but a strict reading pushes toward US self-hosted runners or GovCloud CodePipeline. Needs a deliberate decision.
2. **3PAO selection + timing.** Which assessor, and what PMF threshold triggers the engagement. Ideally tied to a design partner willing to co-commit.
3. **Per-tenant key strategy.** Single CMK with prefix-scoped policies vs per-tenant data keys. Trade-off: blast-radius isolation vs key-management overhead.
4. **800-171 self-attestation interim.** Whether to stand up an interim 800-171 self-assessment (lighter) to support early design-partner conversations before full equivalency.

---

## 11. Resolved decisions (log)

- **Compliance bar:** GovCloud data-handling foundation now, **FedRAMP Moderate equivalency** on PMF signal. Driven by CUI-handling-CSP status under DFARS 7012 / CMMC.
- **Deployment model:** **multi-tenant SaaS**, RapidDFM as CSP of record. Enclave model rejected because small-subcontractor alpha/core market lacks enclaves to inherit.
- **Alpha data:** **non-CUI / synthetic** boards only, with a hard CUI-upload guardrail until equivalency lands.
- **OpenAI:** **removed entirely**, no Bedrock replacement. Deterministic fallback covers the feature.

---

## 12. Explicitly out of scope

- Full FedRAMP Moderate **authorization** (marketplace listing, agency sponsor): a later graduation, not now.
- CMMC certification of RapidDFM itself (we enable customers' CMMC; we are assessed as their CSP via FedRAMP Moderate equivalency).
- Bedrock or any in-boundary AI replacement for the removed OpenAI feature.
- Multi-region / DR posture beyond what GovCloud single-region provides.

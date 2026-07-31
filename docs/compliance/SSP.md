# System Security Plan (SSP) — RapidDFM GovCloud

**System:** RapidDFM — Design-for-Manufacturability analysis platform (GovCloud / ITAR-CUI edition)
**Prepared by:** Engineering (with system-verified control evidence)
**Framework:** NIST SP 800-171 Rev. 2 (110 controls) — the DFARS 252.204-7012 / CMMC Level 2 baseline
**Status:** DRAFT for review
**Last updated:** 2026-07-31

> **Read this first.** This SSP was drafted from direct inspection of the deployed
> GovCloud environment. It is an engineering work product, **not** a substitute for
> review by a qualified ISSO/assessor or export-control counsel. Control statements
> reflect the system state as verified on the date above; the companion **POA&M**
> (§5) tracks every gap. Do not represent this as an assessed or certified document.

---

## 1. System Identification

| Field | Value |
|---|---|
| System name | RapidDFM (GovCloud edition) |
| Purpose | Contract manufacturers upload PCB design data (ODB++); the system parses geometry, runs manufacturability rule checks, and presents violations. |
| Information type | Export-controlled (ITAR) technical data — CUI category "Export Controlled" (marking `CUI//SP-EXPT`). PCB design files and derived analysis. |
| Deployment | AWS GovCloud (US), region `us-gov-west-1`, account `665462955903`, partition `aws-us-gov`. |
| Separation | Physically and logically separate AWS account/partition from the commercial (Vercel/commercial-AWS) edition. No shared data plane. |
| Access model | Invite-only; US-persons only (see §3, §4 AC/PS). No public self-signup. |
| System owner | Basel Anani |
| ISSO | [To be designated] |

### 1.1 Authorization boundary

The boundary is the single GovCloud account `665462955903` in `us-gov-west-1`, comprising:
the VPC (public/private subnets), Application Load Balancer, four ECS/Fargate services
(web, api, worker, gerbonara), RDS PostgreSQL, S3 (uploads + audit), SQS, Cognito user
pool, KMS CMK, and CloudTrail/CloudWatch logging. Everything that stores, processes, or
transmits CUI lives inside this boundary. The commercial edition is **out of boundary**.

---

## 2. System Environment & Architecture

### 2.1 Components

| Component | Role | Notes |
|---|---|---|
| **web** (Next.js, ECS) | UI + auth broker | Public via ALB (HTTPS). Brokers Cognito auth incl. MFA. |
| **api** (Go, ECS) | REST API, authZ, presigned URLs | Behind ALB. Validates Cognito JWT; enforces org isolation + RBAC. |
| **worker** (Go, ECS) | Async analysis pipeline | Consumes SQS; calls gerbonara; runs the DFM engine; writes results. |
| **gerbonara** (Python, ECS) | ODB++ parser | Internal only. Downloads from S3, parses geometry. |
| **RDS PostgreSQL** | Primary datastore | Encrypted (CMK), private subnet, deletion-protected, 7-day backups. |
| **S3 (uploads, cloudtrail)** | Object storage + audit archive | SSE-KMS (CMK), versioned. |
| **SQS (jobs + DLQ)** | Job queue | KMS-encrypted (CMK). |
| **Cognito user pool** | Identity provider | JWT (RS256), pool-wide MFA (TOTP) enforced, custom orgId/role claims. |
| **KMS CMK** | Key management | FIPS 140-validated; encrypts RDS, S3, SQS, CloudTrail. |
| **CloudTrail + CloudWatch** | Audit | Multi-region trail, log-file validation, dual S3+CloudWatch delivery. |

### 2.2 Data flow

Browser → (HTTPS/ALB) → web → api → presigned S3 URL → browser uploads ODB++ directly to
S3 (SSE-KMS) → api enqueues SQS job → worker consumes → worker → gerbonara (parse) →
worker runs DFM engine → results written to RDS + S3 → browser polls api → renders. All
AWS API calls use FIPS endpoints (§3.13). External ingress is TLS 1.2/1.3.

### 2.3 CUI locations
Uploaded ODB++ (`s3://…-uploads/submissions/`), parsed board + violations
(`…/results/`, RDS), and derived analysis (RDS). All encrypted at rest with the CMK.

---

## 3. Roles & Responsibilities

| Role | Responsibility |
|---|---|
| System Owner / sole engineer | Deployment, configuration, control implementation, incident response. |
| ISSO (to be designated) | Control oversight, audit review, POA&M maintenance. |
| Org Admins (customer) | Manage their own org's users (US-persons attestation — see POA&M). |
| AWS (GovCloud) | Inherited physical, environmental, and infrastructure controls (FedRAMP High authorized). Shared-responsibility model. |

> **Shared responsibility:** physical protection (3.10), environmental controls, and
> hardware maintenance (3.7) are inherited from AWS GovCloud, which is screened to
> US-persons operation and FedRAMP High. Application- and account-level controls are the
> system owner's responsibility and are addressed below.

---

## 4. Control Implementation — NIST SP 800-171 Rev. 2

**Legend:** ✅ Implemented · 🟡 Partially implemented (see POA&M) · 📋 Planned (see POA&M) · 🏛️ Inherited (AWS)

### 3.1 Access Control

| Ctrl | Status | Implementation |
|---|---|---|
| 3.1.1 Authorized access | ✅ | Cognito invite-only pool; every API request requires a valid Cognito JWT validated by the api (signature, issuer, expiry, audience). No anonymous access to CUI. |
| 3.1.2 Limit to permitted transactions | ✅ | Role claim (`custom:role`: ADMIN/ANALYST/VIEWER) enforced by `RequireRole` middleware; org scoping via `custom:orgId` on every query. |
| 3.1.3 Control CUI flow | 🟡 | Egress to third parties removed (analytics hard-disabled in gov; OpenAI egress removed). Internal service hops still plaintext within VPC — **POA&M-01**. |
| 3.1.4 Separation of duties | 🟡 | RBAC separates viewer/analyst/admin. Single-operator environment limits duty separation at the ops layer — **POA&M-06**. |
| 3.1.5 Least privilege | 🟡 | App RBAC least-privilege. Deploy currently uses a broad IAM user rather than a scoped role — **POA&M-02**. |
| 3.1.6 Non-privileged accounts for non-privileged use | ✅ | Admin console is a separate Cognito client + privileged flow; day-to-day app use is non-privileged. |
| 3.1.7 Prevent non-priv users from privileged functions; audit | ✅ | Admin endpoints gated by `AdminMiddleware`; all API actions logged (CloudTrail + app logs). |
| 3.1.8 Limit unsuccessful logon attempts | 🏛️/✅ | Cognito enforces adaptive lockout on repeated failures. |
| 3.1.9 Privacy/security notices | 📋 | Login banner/notice to be added — **POA&M-05**. |
| 3.1.10 Session lock | 🟡 | Token expiry limits session lifetime; explicit inactivity lock in UI — **POA&M-05**. |
| 3.1.11 Session termination | ✅ | Sign-out clears tokens + revokes the refresh cookie; JWT expiry bounds sessions. |
| 3.1.12 Monitor/control remote access | ✅ | All access is via TLS to the ALB; admin/console access authenticated + MFA'd; CloudTrail records control-plane access. |
| 3.1.13 Cryptographic protection of remote access | ✅ | TLS 1.2/1.3 (ALB); FIPS endpoints for AWS control plane. |
| 3.1.14 Route remote access via managed access points | ✅ | Single ALB ingress; RDS/services in private subnets with no public route. |
| 3.1.15 Authorize remote privileged commands | ✅ | Privileged (admin) access requires MFA + admin role; infra changes via authenticated AWS APIs (CloudTrail-logged). |
| 3.1.16–3.1.17 Wireless | N/A | No wireless in the cloud boundary. |
| 3.1.18 Control mobile devices | N/A | No managed mobile devices in boundary. |
| 3.1.19 Encrypt CUI on mobile | N/A | No mobile storage of CUI. |
| 3.1.20 External systems | 🟡 | Third-party egress removed; external connections limited to AWS services (FIPS). Formal external-systems policy — **POA&M-05**. |
| 3.1.21 Limit portable storage | N/A | No portable media in boundary. |
| 3.1.22 Control publicly-accessible content | ✅ | Marketing/landing is the commercial edition (out of boundary); gov app exposes no public CUI. |

### 3.2 Awareness and Training
| 3.2.1 Security awareness | 📋 | Awareness training program to be documented — **POA&M-04**. |
| 3.2.2 Role-based training | 📋 | Same — **POA&M-04**. |
| 3.2.3 Insider-threat awareness | 📋 | Same — **POA&M-04**. |

### 3.3 Audit and Accountability
| 3.3.1 Create/retain audit records | ✅ | CloudTrail (multi-region, all mgmt events) + per-service CloudWatch logs + VPC flow logs; 365-day retention; S3 archive. |
| 3.3.2 Trace actions to users | ✅ | App logs carry user/org; CloudTrail carries IAM principal; Cognito sub in JWT. |
| 3.3.3 Review/update logged events | 📋 | Logged-event set is comprehensive; periodic review of the event definition — **POA&M-03**. |
| 3.3.4 Alert on audit-process failure | 🟡 | CloudTrail delivery monitored; explicit failure alerting to be added — **POA&M-03**. |
| 3.3.5 Correlate audit review | 🟡 | Centralized in CloudWatch; documented correlation/review procedure — **POA&M-03**. |
| 3.3.6 Audit reduction/reporting | ✅ | CloudWatch Logs Insights over centralized groups. |
| 3.3.7 Authoritative time | ✅ | AWS-provided NTP; CloudTrail/CloudWatch timestamps UTC. |
| 3.3.8 Protect audit info | ✅ | CloudTrail log-file validation on; audit S3 + logs encrypted (CMK); access restricted by IAM. |
| 3.3.9 Limit audit management | ✅ | CloudTrail/log config changes are IAM-restricted and CloudTrail-logged. |

### 3.4 Configuration Management
| 3.4.1 Baseline config | ✅ | Infrastructure-as-code (Terraform) defines the baseline; container images are versioned artifacts in ECR. |
| 3.4.2 Enforce config settings | ✅ | Terraform + task definitions enforce settings; immutable image deploys. |
| 3.4.3 Track/review/approve changes | 🟡 | Git history + CloudTrail record changes; formal change-control procedure — **POA&M-05**. |
| 3.4.4 Analyze security impact of changes | 🟡 | Done informally in review; documented procedure — **POA&M-05**. |
| 3.4.5 Access restrictions for change | ✅ | Deploy requires AWS credentials + repo access; production is IaC/CI-gated. |
| 3.4.6 Least functionality | ✅ | Minimal container images (Alpine/distroless-style); only required ports exposed; private subnets. |
| 3.4.7 Restrict nonessential functions | ✅ | Security groups restrict to required ports; no shell/exec enabled on tasks by default. |
| 3.4.8 Application allow/deny | ✅ | Containers run fixed images; no arbitrary code execution paths exposed. |
| 3.4.9 User-installed software | N/A | No user-installed software in the managed boundary. |

### 3.5 Identification and Authentication
| 3.5.1 Identify users/devices | ✅ | Cognito unique identities (sub); every user provisioned admin-side with email + org + role. |
| 3.5.2 Authenticate identities | ✅ | Cognito password auth + mandatory TOTP MFA; JWT RS256 validated server-side. |
| 3.5.3 **MFA** for privileged + network access | ✅ | **Pool-wide `mfa_configuration = ON` (TOTP)**; enrollment + challenge implemented in both app and admin flows; verified end-to-end. Admin (privileged) access is MFA'd. |
| 3.5.4 Replay-resistant auth | ✅ | TOTP (time-based, single-use) + short-lived JWTs; TLS. |
| 3.5.5 Prevent identifier reuse | ✅ | Cognito sub is unique/non-reused; emails unique per pool. |
| 3.5.6 Disable inactive identifiers | 🟡 | Admin can disable/delete users; automated inactivity disablement — **POA&M-05**. |
| 3.5.7–3.5.9 Password complexity/reuse/temporary | ✅ | Policy: ≥14 chars, upper/lower/number/symbol; temp passwords expire in 7 days; new-password-required on first login. |
| 3.5.10 Cryptographically-protected passwords | ✅ | Cognito stores credentials hashed; never handled in plaintext by the app. |
| 3.5.11 Obscure authentication feedback | ✅ | Generic "incorrect email or password" responses; masked inputs. |

### 3.6 Incident Response
| 3.6.1 IR capability | 🟡 | Incident-response baseline documented (`docs/INCIDENT-RESPONSE.md`); tested runbook — **POA&M-04**. |
| 3.6.2 Track/report incidents | 🟡 | Reporting flow to be finalized (incl. DoD 72-hour reporting for CUI incidents) — **POA&M-04**. |
| 3.6.3 Test IR | 📋 | Tabletop test to be scheduled — **POA&M-04**. |

### 3.7 Maintenance
| 3.7.1–3.7.6 | 🏛️/✅ | Hardware maintenance inherited from AWS GovCloud. Software maintenance via IaC + immutable image redeploys; no remote maintenance tools with CUI access. |

### 3.8 Media Protection
| 3.8.1 Protect media | ✅ | All CUI at rest is SSE-KMS (S3, RDS, SQS) with the CMK; no physical media. |
| 3.8.2 Limit access to media | ✅ | IAM + bucket policies restrict access; RDS in private subnet. |
| 3.8.3 Sanitize media before disposal | 🏛️ | AWS media sanitization (inherited); CUI deletion removes objects + versions (demonstrated). |
| 3.8.4 Mark media | 🟡 | Data classified as CUI at the system level; per-object marking — **POA&M-05**. |
| 3.8.5 Control media access/transport | ✅ | No removable media; all transport is encrypted network transfer. |
| 3.8.6 Cryptographic protection on transport | ✅ | TLS in transit; SSE-KMS at rest. |
| 3.8.7 Control removable media | N/A | None in boundary. |
| 3.8.8 Prohibit unauthorized portable storage | N/A | None in boundary. |
| 3.8.9 Protect backups | ✅ | RDS automated backups encrypted (CMK); S3 versioned + encrypted. |

### 3.9 Personnel Security
| 3.9.1 Screen personnel | 🟡 | US-persons requirement defined; formal screening/attestation procedure (incl. provisioning-time capture) — **POA&M-05**. |
| 3.9.2 Protect CUI on personnel actions | ✅ | Admin can immediately disable/delete a user (Cognito + DB); demonstrated. |

### 3.10 Physical Protection
| 3.10.1–3.10.6 | 🏛️ | Inherited from AWS GovCloud data centers (US-persons-operated, FedRAMP High). No system-owner physical facility hosts CUI. |

### 3.11 Risk Assessment
| 3.11.1 Assess risk | 🟡 | This SSP + POA&M constitute the initial risk view; formal periodic risk assessment — **POA&M-04**. |
| 3.11.2 Scan for vulnerabilities | 🟡 | Base images from maintained upstreams; dependency + image scanning to be formalized — **POA&M-03**. |
| 3.11.3 Remediate vulnerabilities | 🟡 | Patch via image rebuild/redeploy; documented SLA — **POA&M-03**. |

### 3.12 Security Assessment
| 3.12.1 Assess controls | 🟡 | This SSP is the self-assessment basis; formal 800-171 self-assessment + SPRS score — **POA&M-04**. |
| 3.12.2 Plan of action | ✅ | POA&M maintained (§5). |
| 3.12.3 Monitor controls | 🟡 | CloudTrail/CloudWatch provide continuous monitoring; documented ConMon plan — **POA&M-04**. |
| 3.12.4 System security plan | ✅ | This document. |

### 3.13 System and Communications Protection
| 3.13.1 Monitor/control boundary comms | ✅ | Single ALB ingress; security groups + private subnets; egress restricted; VPC flow logs. |
| 3.13.2 Architectural security | ✅ | Tiered architecture; least-functionality images; IaC-defined. |
| 3.13.3 Separate user/management functions | ✅ | Admin console (mgmt) separated from app (user) via distinct clients + flows. |
| 3.13.4 Prevent unauthorized info transfer via shared resources | ✅ | Per-org isolation enforced in code; authz tests prove cross-org reads are blocked. |
| 3.13.5 Public-access subnetworks | ✅ | Only ALB in public subnets; all compute/data in private subnets. |
| 3.13.6 Deny-by-default network comms | ✅ | Security groups deny by default; only required flows allowed. |
| 3.13.7 Prevent split tunneling | N/A | No VPN clients in boundary. |
| 3.13.8 **Encrypt CUI in transit** | 🟡 | External TLS 1.2/1.3; all AWS API calls FIPS. **Internal service-to-service traffic is plaintext within the VPC** — **POA&M-01**. |
| 3.13.9 Terminate network connections | ✅ | Idle connection termination at ALB; session/JWT expiry. |
| 3.13.10 Key management | ✅ | KMS CMK (FIPS 140-validated); automatic rotation configurable; keys never exposed to the app. |
| 3.13.11 **FIPS-validated cryptography** | ✅ | FIPS endpoints enforced across all services (Go region-forced; Python boto3 `use_fips_endpoint`; web `AWS_USE_FIPS_ENDPOINT=true`); KMS is FIPS 140-validated. Verified 2026-07-31. |
| 3.13.12 Collaborative device control | N/A | None in boundary. |
| 3.13.13 Mobile code control | ✅ | Frontend is first-party; strict content; no third-party trackers in gov (analytics hard-disabled). |
| 3.13.14 VoIP | N/A | None. |
| 3.13.15 Authenticity of communications | ✅ | TLS server auth; JWT signature validation; CloudTrail log-file validation. |
| 3.13.16 Protect CUI at rest | ✅ | SSE-KMS (CMK) on S3, RDS, SQS. |

### 3.14 System and Information Integrity
| 3.14.1 Flaw remediation | 🟡 | Patch via redeploy; documented remediation SLA — **POA&M-03**. |
| 3.14.2 Malicious-code protection | 🟡 | Minimal images reduce surface; upload archive extraction hardened (`filter="data"`, no path traversal); AV/scanning of uploads — **POA&M-03**. |
| 3.14.3 Monitor security alerts | 🟡 | CloudWatch alarms baseline; formal alert-intake procedure — **POA&M-03**. |
| 3.14.4 Update malicious-code protection | 🟡 | Tied to 3.14.2 — **POA&M-03**. |
| 3.14.5 Periodic + real-time scans | 🟡 | Upload validation in place; scheduled scanning — **POA&M-03**. |
| 3.14.6 Monitor communications for attacks | ✅ | VPC flow logs + CloudTrail; ALB access logging available. |
| 3.14.7 Identify unauthorized use | ✅ | Auth logs + org-scoped access + anomaly-visible audit trail. |

---

## 5. Plan of Action & Milestones (POA&M)

| ID | Gap | Control(s) | Planned action | Owner | Target |
|---|---|---|---|---|---|
| POA&M-01 | Internal service-to-service traffic is plaintext HTTP within the VPC | 3.13.8, 3.1.3 | Implement internal encryption (internal ACM/TLS or mTLS/service mesh) between web↔api and worker↔gerbonara | Basel Anani | [date] |
| POA&M-02 | Deploy uses a broad IAM user, not a scoped role | 3.1.5 | Move deploys to the least-privilege OIDC deploy role; retire the standing admin user for routine deploys | Basel Anani | [date] |
| POA&M-03 | Vulnerability scanning, flaw-remediation SLA, upload AV, audit-review procedure not formalized | 3.3.3–3.3.5, 3.11.2–3.11.3, 3.14.1–3.14.5 | Add dependency/image scanning + upload scanning; document remediation SLA and log-review procedure | Basel Anani | [date] |
| POA&M-04 | Awareness training, IR test, formal risk + security self-assessment (SPRS) not yet done | 3.2.x, 3.6.x, 3.11.1, 3.12.1/3.12.3 | Complete awareness training, IR tabletop, risk assessment, and 800-171 self-assessment → SPRS score | Basel Anani | [date] |
| POA&M-05 | Policy set + provisioning-time US-persons attestation + login banner + change-control docs | 3.1.9/3.1.10/3.1.20, 3.4.3/3.4.4, 3.5.6, 3.8.4, 3.9.1 | Author policies (access, retention, change mgmt, media marking); add US-persons attestation at user creation; add login notice | Basel Anani | [date] |
| POA&M-06 | Single-operator limits separation of duties | 3.1.4 | Compensating controls (full audit logging, MFA) documented; revisit as team grows | Basel Anani | [date] |

---

## 6. Control status summary

| Family | ✅ | 🟡 | 📋 | 🏛️/NA |
|---|---|---|---|---|
| 3.1 Access Control | 11 | 5 | 1 | 5 |
| 3.2 Awareness/Training | 0 | 0 | 3 | 0 |
| 3.3 Audit | 6 | 3 | 0 | 0 |
| 3.4 Config Mgmt | 6 | 2 | 0 | 1 |
| 3.5 Identification/Auth | 9 | 2 | 0 | 0 |
| 3.6 Incident Response | 0 | 2 | 1 | 0 |
| 3.7 Maintenance | — | — | — | 6 |
| 3.8 Media | 6 | 1 | 0 | 2 |
| 3.9 Personnel | 1 | 1 | 0 | 0 |
| 3.10 Physical | — | — | — | 6 |
| 3.11 Risk Assessment | 0 | 3 | 0 | 0 |
| 3.12 Security Assessment | 2 | 2 | 0 | 0 |
| 3.13 System/Comms | 14 | 1 | 0 | 1 |
| 3.14 System Integrity | 2 | 5 | 0 | 0 |

**Headline:** the core technical controls that assessors weight heavily — access control,
MFA, FIPS cryptography, audit logging, boundary protection, encryption at rest — are
**implemented and verified**. Open items are concentrated in **documentation/process**
(policies, training, formal assessment) and **one real technical gap** (internal TLS,
POA&M-01). None of the open items block operating with a clean POA&M, subject to
ISSO/counsel review and the export-control determination (§0).

---

*End of SSP draft. Bracketed fields ([Owner], [date], names) require completion by the system owner/ISSO.*

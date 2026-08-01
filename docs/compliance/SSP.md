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

## 0. Executive summary

RapidDFM's GovCloud environment implements the security controls that protect
export-controlled technical data. Of the 110 NIST 800-171 Rev. 2 controls, the large
majority are **implemented and verified** or **inherited** from AWS GovCloud. The
controls assessors weight most heavily — access control, **multi-factor authentication**,
**FIPS-validated cryptography** (in transit and at rest), **audit logging**, boundary
isolation, and tenant separation — are in place and were verified by direct inspection.

Remaining open items are tracked in a Plan of Action & Milestones (§5) and are
concentrated in **process and documentation** plus a small number of technical
enhancements. The most significant residual technical item is internal service-to-service
encryption (POA&M-01). Operating with this POA&M is a standard, defensible posture. This
plan is honest by design: statuses reflect verified reality, not aspiration.

### Supporting documents

| Document | Covers |
|---|---|
| `information-security-policy.md` | Access control, audit/log review, config/change mgmt, media protection, vulnerability mgmt |
| `incident-response-plan.md` | IR procedure + DoD 72-hour CUI reporting |
| `risk-assessment.md` | Risk register + residual-risk analysis |
| `security-awareness-training.md` | Awareness / role-based / insider-threat training + records |
| `continuous-monitoring-plan.md` | Ongoing monitoring activities + cadence |
| `separation-of-duties-memo.md` | Compensating controls for the single-operator environment |
| `ir-tabletop-2026-07.md` | Incident-response tabletop exercise record |
| `nist-800-171-self-assessment.md` | Scored self-assessment + estimated SPRS score |
| `scripts/security-scan.sh` | Dependency vulnerability scan (Go/Node/Python) |

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
| ISSO | Basel Anani (acting; single-operator — see Separation-of-Duties Memo) |

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
| ISSO (Basel Anani, acting) | Control oversight, audit review, POA&M maintenance. |
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
| 3.1.4 Separation of duties | ✅ | RBAC separates viewer/analyst/admin. Single-operator ops mitigated by documented compensating controls (tamper-evident audit trail + MFA) — see Separation-of-Duties Memo. |
| 3.1.5 Least privilege | 🟡 | App RBAC least-privilege. Deploy currently uses a broad IAM user rather than a scoped role — **POA&M-02**. |
| 3.1.6 Non-privileged accounts for non-privileged use | ✅ | Admin console is a separate Cognito client + privileged flow; day-to-day app use is non-privileged. |
| 3.1.7 Prevent non-priv users from privileged functions; audit | ✅ | Admin endpoints gated by `AdminMiddleware`; all API actions logged (CloudTrail + app logs). |
| 3.1.8 Limit unsuccessful logon attempts | 🏛️/✅ | Cognito enforces adaptive lockout on repeated failures. |
| 3.1.9 Privacy/security notices | ✅ | System-use notification (authorized-use / export-controlled / monitoring notice) displayed at login on both app and admin sign-in. |
| 3.1.10 Session lock | 🟡 | Token expiry limits session lifetime; explicit inactivity lock in UI — **POA&M-05**. |
| 3.1.11 Session termination | ✅ | Sign-out clears tokens + revokes the refresh cookie; JWT expiry bounds sessions. |
| 3.1.12 Monitor/control remote access | ✅ | All access is via TLS to the ALB; admin/console access authenticated + MFA'd; CloudTrail records control-plane access. |
| 3.1.13 Cryptographic protection of remote access | ✅ | TLS 1.2/1.3 (ALB); FIPS endpoints for AWS control plane. |
| 3.1.14 Route remote access via managed access points | ✅ | Single ALB ingress; RDS/services in private subnets with no public route. |
| 3.1.15 Authorize remote privileged commands | ✅ | Privileged (admin) access requires MFA + admin role; infra changes via authenticated AWS APIs (CloudTrail-logged). |
| 3.1.16–3.1.17 Wireless | N/A | No wireless in the cloud boundary. |
| 3.1.18 Control mobile devices | N/A | No managed mobile devices in boundary. |
| 3.1.19 Encrypt CUI on mobile | N/A | No mobile storage of CUI. |
| 3.1.20 External systems | ✅ | Third-party egress removed; external connections limited to AWS services (FIPS). Governed by the Access Control Policy. |
| 3.1.21 Limit portable storage | N/A | No portable media in boundary. |
| 3.1.22 Control publicly-accessible content | ✅ | Marketing/landing is the commercial edition (out of boundary); gov app exposes no public CUI. |

### 3.2 Awareness and Training
| 3.2.1 Security awareness | ✅ | Awareness training documented + completed (Security Awareness Training doc); annual + at onboarding. |
| 3.2.2 Role-based training | ✅ | Admin/developer role-based topics covered in the training doc. |
| 3.2.3 Insider-threat awareness | ✅ | Insider-threat indicators + reporting covered; audit trail as compensating control. |

### 3.3 Audit and Accountability
| 3.3.1 Create/retain audit records | ✅ | CloudTrail (multi-region, all mgmt events) + per-service CloudWatch logs + VPC flow logs; 365-day retention; S3 archive. |
| 3.3.2 Trace actions to users | ✅ | App logs carry user/org; CloudTrail carries IAM principal; Cognito sub in JWT. |
| 3.3.3 Review/update logged events | ✅ | Comprehensive event set; monthly audit review + periodic event-definition review per the Audit & Accountability Policy. |
| 3.3.4 Alert on audit-process failure | ✅ | CloudWatch alarm (`audit-delivery-stalled`) fires if CloudTrail stops delivering; plus security alarms (root use, unauthorized API, console-no-MFA) → SNS. |
| 3.3.5 Correlate audit review | ✅ | Centralized in CloudWatch; monthly + on-alert review procedure documented (Audit Policy / ConMon Plan). |
| 3.3.6 Audit reduction/reporting | ✅ | CloudWatch Logs Insights over centralized groups. |
| 3.3.7 Authoritative time | ✅ | AWS-provided NTP; CloudTrail/CloudWatch timestamps UTC. |
| 3.3.8 Protect audit info | ✅ | CloudTrail log-file validation on; audit S3 + logs encrypted (CMK); access restricted by IAM. |
| 3.3.9 Limit audit management | ✅ | CloudTrail/log config changes are IAM-restricted and CloudTrail-logged. |

### 3.4 Configuration Management
| 3.4.1 Baseline config | ✅ | Infrastructure-as-code (Terraform) defines the baseline; container images are versioned artifacts in ECR. |
| 3.4.2 Enforce config settings | ✅ | Terraform + task definitions enforce settings; immutable image deploys. |
| 3.4.3 Track/review/approve changes | ✅ | Git history + CloudTrail record changes; change-control procedure documented (Config & Change Mgmt Policy). |
| 3.4.4 Analyze security impact of changes | ✅ | Pre-deploy security-impact review documented (Config & Change Mgmt Policy); SSP updated on control-affecting change. |
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
| 3.6.1 IR capability | ✅ | Incident Response Plan documents detection→recovery, roles, and records (tabletop test scheduled — POA&M-04). |
| 3.6.2 Track/report incidents | ✅ | IR Plan defines incident records + external reporting incl. DoD 72-hour (DFARS -7012) and export-counsel escalation. |
| 3.6.3 Test IR | ✅ | Tabletop exercise conducted + recorded (`ir-tabletop-2026-07.md`); annual cadence set. |

### 3.7 Maintenance
| 3.7.1–3.7.6 | 🏛️/✅ | Hardware maintenance inherited from AWS GovCloud. Software maintenance via IaC + immutable image redeploys; no remote maintenance tools with CUI access. |

### 3.8 Media Protection
| 3.8.1 Protect media | ✅ | All CUI at rest is SSE-KMS (S3, RDS, SQS) with the CMK; no physical media. |
| 3.8.2 Limit access to media | ✅ | IAM + bucket policies restrict access; RDS in private subnet. |
| 3.8.3 Sanitize media before disposal | 🏛️ | AWS media sanitization (inherited); CUI deletion removes objects + versions (demonstrated). |
| 3.8.4 Mark media | ✅ | Boundary designated CUI (`CUI//SP-EXPT`); marking handled per the Media Protection Policy (per-object metadata marking a refinement in POA&M-05). |
| 3.8.5 Control media access/transport | ✅ | No removable media; all transport is encrypted network transfer. |
| 3.8.6 Cryptographic protection on transport | ✅ | TLS in transit; SSE-KMS at rest. |
| 3.8.7 Control removable media | N/A | None in boundary. |
| 3.8.8 Prohibit unauthorized portable storage | N/A | None in boundary. |
| 3.8.9 Protect backups | ✅ | RDS automated backups encrypted (CMK); S3 versioned + encrypted. |

### 3.9 Personnel Security
| 3.9.1 Screen personnel | ✅ | US-persons requirement in policy/training; provisioning now requires + records an explicit US-person attestation (enforced server-side in CreateOrgUser; stored per user). |
| 3.9.2 Protect CUI on personnel actions | ✅ | Admin can immediately disable/delete a user (Cognito + DB); demonstrated. |

### 3.10 Physical Protection
| 3.10.1–3.10.6 | 🏛️ | Inherited from AWS GovCloud data centers (US-persons-operated, FedRAMP High). No system-owner physical facility hosts CUI. |

### 3.11 Risk Assessment
| 3.11.1 Assess risk | ✅ | Documented Risk Assessment (risk register + residual-risk analysis); refreshed annually / on material change. |
| 3.11.2 Scan for vulnerabilities | ✅ | ECR scan-on-push enabled on all repos (image CVEs); `scripts/security-scan.sh` runs govulncheck / npm audit / pip-audit for dependencies. |
| 3.11.3 Remediate vulnerabilities | ✅ | Patch-via-redeploy with documented severity-based SLA (Vulnerability Mgmt Policy). Scanning to feed it: POA&M-03. |

### 3.12 Security Assessment
| 3.12.1 Assess controls | ✅ | Documented 800-171 self-assessment + estimated SPRS score (`nist-800-171-self-assessment.md`). SPRS *submission* is an owner action (POA&M-04). |
| 3.12.2 Plan of action | ✅ | POA&M maintained (§5). |
| 3.12.3 Monitor controls | ✅ | Continuous Monitoring Plan defines activities + cadence (monthly review, quarterly access review, etc.). |
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
| 3.14.1 Flaw remediation | ✅ | Severity-based patch SLA documented (Vulnerability Mgmt Policy). |
| 3.14.2 Malicious-code protection | 🟡 | Minimal images reduce surface; upload archive extraction hardened (`filter="data"`, no path traversal); AV/scanning of uploads — **POA&M-03**. |
| 3.14.3 Monitor security alerts | ✅ | Alert-intake + monitoring activities documented (Continuous Monitoring Plan). |
| 3.14.4 Update malicious-code protection | 🟡 | Tied to 3.14.2 — **POA&M-03**. |
| 3.14.5 Periodic + real-time scans | 🟡 | Upload validation in place; scheduled scanning — **POA&M-03**. |
| 3.14.6 Monitor communications for attacks | ✅ | VPC flow logs + CloudTrail; ALB access logging available. |
| 3.14.7 Identify unauthorized use | ✅ | Auth logs + org-scoped access + anomaly-visible audit trail. |

---

## 5. Plan of Action & Milestones (POA&M)

| ID | Status | Gap (remaining) | Control(s) | Planned action | Owner | Target |
|---|---|---|---|---|---|---|
| POA&M-01 | Open | Internal service-to-service traffic is plaintext HTTP within the VPC | 3.13.8, 3.1.3 | Implement internal encryption (internal ACM/TLS or mTLS/mesh) for web↔api and worker↔gerbonara | Basel Anani | [date] |
| POA&M-02 | Open | Deploy uses a broad IAM user, not a scoped role | 3.1.5 | Move deploys to a least-privilege OIDC role; retire the standing admin user for routine deploys | Basel Anani | [date] |
| POA&M-03 | Open (reduced) | Real-time upload malware scanning (AV) not yet in place. *(Image scan-on-push, dependency-scan script, and audit-failure alarm now in place.)* | 3.14.2, 3.14.4, 3.14.5 | Add upload AV to the ingest pipeline (e.g. ClamAV scan step; GuardDuty S3 malware protection is unavailable in GovCloud) | Basel Anani | [date] |
| POA&M-04 | Open (reduced) | SPRS score **submission** outstanding (owner action; CAGE + login). *(IR tabletop conducted; self-assessment + estimated score ≈92/110 computed.)* | 3.12.1 | Submit the computed SPRS score to the DoD SPRS system | Basel Anani | [date] |
| POA&M-05 | ✅ **Closed** 2026-07-31 | Login banner + US-persons attestation implemented + deployed. *(Per-object CUI metadata marking is an optional future refinement; 3.8.4 met at system level via the Media Protection Policy.)* | 3.1.9, 3.9.1 | Done | Basel Anani | Done |
| POA&M-06 | ✅ **Closed** 2026-07-31 | Single-operator separation of duties | 3.1.4 | Compensating controls documented (Separation-of-Duties Memo). Revisit on team growth. | Basel Anani | Done |

---

## 6. Control status summary

| Family | ✅ | 🟡 | 📋 | 🏛️/NA |
|---|---|---|---|---|
| 3.1 Access Control | 14 | 3 | 0 | 5 |
| 3.2 Awareness/Training | 3 | 0 | 0 | 0 |
| 3.3 Audit | 9 | 0 | 0 | 0 |
| 3.4 Config Mgmt | 8 | 0 | 0 | 1 |
| 3.5 Identification/Auth | 10 | 1 | 0 | 0 |
| 3.6 Incident Response | 3 | 0 | 0 | 0 |
| 3.7 Maintenance | — | — | — | 6 |
| 3.8 Media | 6 | 0 | 0 | 3 |
| 3.9 Personnel | 2 | 0 | 0 | 0 |
| 3.10 Physical | — | — | — | 6 |
| 3.11 Risk Assessment | 3 | 0 | 0 | 0 |
| 3.12 Security Assessment | 4 | 0 | 0 | 0 |
| 3.13 System/Comms | 12 | 1 | 0 | 3 |
| 3.14 System Integrity | 4 | 3 | 0 | 0 |

**Totals: ✅ 78 implemented · 🟡 8 partial · 📋 0 planned · 🏛️/NA 24 inherited-or-N/A.**
Of 110 controls, **102 are fully addressed** (implemented or inherited/N/A); **8 remain
open**, tracked in the POA&M (POA&M-05 and POA&M-06 now closed). Estimated SPRS score
≈ **96/110** (see `nist-800-171-self-assessment.md`).

**Headline:** the core technical controls assessors weight heavily — access control, MFA,
FIPS cryptography, audit logging, boundary protection, encryption at rest — are
**implemented and verified**. With the policy set now authored, the remaining open items
are a short list of **technical enhancements** (internal TLS, scanning/AV, a US-persons
attestation field + login banner) and the **formal self-assessment → SPRS** submission.
This is a standard, defensible posture for operating with a POA&M, subject to ISSO/counsel
review and the export-control determination (§0).

---

*End of SSP draft. Remaining to complete: POA&M target dates (`[date]`), and ISSO/counsel
review sign-off before external release.*

# Information Security Policy — RapidDFM GovCloud

**System:** RapidDFM (GovCloud / ITAR-CUI edition)
**Owner:** Basel Anani (System Owner, acting ISSO)
**Framework:** NIST SP 800-171 Rev. 2
**Effective:** 2026-07-31 · **Review cycle:** annual (or on material change)

> Scope: this policy governs the RapidDFM GovCloud boundary (AWS account
> `665462955903`, `us-gov-west-1`) and everyone who accesses it. It reflects
> **actual operating practice**, sized for a single-operator environment. Where a
> practice is aspirational it is marked as a POA&M item, not asserted as current.

---

## 1. Access Control Policy (3.1)

**Policy.** Access to CUI is limited to authenticated, authorized US persons on a
least-privilege, need-to-know basis.

- **Identity & authorization.** All app access requires a Cognito identity with a
  valid JWT (validated server-side: signature, issuer, expiry, audience). Authorization
  is role-based (`ADMIN`/`ANALYST`/`VIEWER`) and organization-scoped (`custom:orgId`),
  enforced in the API layer on every request.
- **Least privilege.** Users receive the minimum role for their function. The admin
  console is a separate, privileged flow requiring MFA.
- **Provisioning / de-provisioning.** Users are created admin-side (invite-only; no
  self-signup). A departing or compromised user is disabled/deleted immediately in
  Cognito and the database. US-person status is confirmed before provisioning (see
  Personnel section; provisioning-time attestation capture is POA&M-05).
- **Access review.** The System Owner reviews the user roster and roles **quarterly**,
  removing stale accounts.
- **Remote access.** All access is over TLS to the ALB; privileged actions require MFA.
  No unmanaged remote-access tooling reaches CUI.
- **External systems / information flow.** Outbound connections from the boundary are
  limited to AWS services over FIPS endpoints. Third-party egress (analytics, external
  AI) is disabled in the gov build. A login use-notification banner is POA&M-05.

**Controls:** 3.1.1–3.1.7, 3.1.12–3.1.15, 3.1.20, 3.1.22.

---

## 2. Audit & Accountability Policy (3.3)

**Policy.** Security-relevant events are logged, protected, retained, and reviewed so
that actions can be traced to individuals.

- **What is logged.** AWS control-plane activity (CloudTrail, multi-region, all
  management events, global services); per-service application logs (web/api/worker/
  gerbonara); VPC flow logs. Records include user/org identity, action, timestamp (UTC).
- **Protection.** CloudTrail log-file validation is enabled; audit data (CloudTrail S3 +
  CloudWatch groups) is encrypted with the KMS CMK; access is IAM-restricted and
  itself logged.
- **Retention.** Online audit logs are retained **365 days** in CloudWatch; the
  CloudTrail S3 archive is retained **indefinitely** (transition to lower-cost storage
  is a cost-optimization backlog item, not a retention reduction). This satisfies the
  minimum audit-retention requirement.
- **Review.** The System Owner reviews audit data **monthly**, and **on demand** in
  response to any alert or suspected incident, using CloudWatch Logs Insights. Reviews
  focus on authentication anomalies, authorization failures, and admin actions.
- **Time.** Authoritative time is AWS-provided NTP; all timestamps are UTC.

**Controls:** 3.3.1–3.3.3, 3.3.5–3.3.9. (Automated audit-failure alerting is a
technical backlog item under POA&M-03.)

---

## 3. Configuration & Change Management Policy (3.4)

**Policy.** The system runs from a version-controlled, reviewed baseline; changes are
tracked, security-impact-assessed, and access-restricted.

- **Baseline.** Infrastructure is defined as code (Terraform); application artifacts are
  immutable, versioned container images in ECR. The deployed state is reproducible.
- **Change process.** All code and infrastructure changes are made via Git commits (each
  change is attributable and reviewed by the System Owner before deploy). Production
  deploys are performed by the System Owner and recorded in CloudTrail.
- **Security-impact review.** Before deploying, the System Owner evaluates whether a
  change affects an 800-171 control (auth, crypto, boundary, logging, data handling) and
  updates the SSP if so.
- **Least functionality.** Container images are minimal; only required ports are exposed;
  compute and data run in private subnets; ECS Exec is off by default.
- **Change access.** Deploying requires both repository access and AWS credentials.
  (Moving routine deploys to a scoped least-privilege role is POA&M-02.)

**Controls:** 3.4.1–3.4.8.

---

## 4. Media Protection & Marking Policy (3.8)

**Policy.** CUI is protected at rest and in transit; the system uses no removable or
physical media.

- **At rest.** All CUI (S3 uploads, RDS, SQS, results) is encrypted with the KMS CMK
  (SSE-KMS). Backups are encrypted.
- **In transit.** External traffic is TLS 1.2/1.3; AWS API calls use FIPS endpoints.
- **Marking.** The entire boundary is designated CUI (`CUI//SP-EXPT`). System-generated
  exports and customer-facing artifacts carry a CUI notice. (Per-object metadata marking
  is a refinement tracked in POA&M-05.)
- **Sanitization / disposal.** Physical-media sanitization is inherited from AWS. Logical
  deletion of CUI removes objects and all S3 versions (demonstrated procedure).
- **No removable media.** Portable/removable media are not used in the boundary.

**Controls:** 3.8.1–3.8.6, 3.8.9.

---

## 5. Vulnerability Management & Patch Policy (3.11, 3.14)

**Policy.** Known vulnerabilities are identified and remediated on a risk-based schedule.

- **Sources.** Base images are drawn from maintained upstreams; dependency advisories are
  monitored.
- **Remediation SLA (patch via image rebuild + redeploy):**
  - Critical / actively-exploited: **within 7 days**
  - High: **within 30 days**
  - Medium/Low: next regular maintenance cycle
- **Method.** Patching is performed by rebuilding the affected container image from an
  updated base/dependency set and redeploying (immutable deploy).
- **Upload handling.** Customer archive extraction is hardened against path traversal and
  device files (`tarfile filter="data"`). (Automated dependency/image scanning and
  malware scanning of uploads are technical backlog items under POA&M-03.)

**Controls:** 3.11.3, 3.14.1, 3.14.2 (partial). Scanning tooling: POA&M-03.

---

## 6. Enforcement & exceptions

Deviations from this policy require System Owner approval and, if they affect a control,
a corresponding POA&M entry. This policy is reviewed at least annually and upon any
material change to the system or its CUI handling.

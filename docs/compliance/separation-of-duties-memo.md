# Separation of Duties — Compensating Controls Memo

**System:** RapidDFM (GovCloud / ITAR-CUI edition)
**Owner:** Basel Anani · **Date:** 2026-07-31 · **Review:** annual / on team change
**Control:** 3.1.4 (separate the duties of individuals to reduce risk of malevolent
activity without collusion)

---

## 1. Situation

RapidDFM's GovCloud environment currently operates with a **single individual** (the
System Owner) performing development, deployment, and administration. Traditional
separation of duties — dividing request/approval/execution among distinct people — is
not achievable with one person. NIST 800-171 permits **compensating controls** where a
control cannot be applied as written.

## 2. Compensating controls in effect

| Compensating control | How it mitigates the lack of SoD |
|---|---|
| **Complete, tamper-evident audit trail** | Every action (code, infra, admin, data) is attributable and logged: Git history, CloudTrail (multi-region, **log-file validation on**), and application logs. A sole operator cannot act without leaving an immutable record. |
| **MFA on all access, incl. privileged** | Enforced TOTP MFA (pool-wide `ON`) prevents credential-only compromise from impersonating the operator. |
| **Infrastructure as code + immutable deploys** | Changes are reviewed as version-controlled commits before deploy; production state is reproducible and diffable, not ad-hoc. |
| **Least functionality / least privilege in-app** | RBAC + org isolation limit blast radius; ECS Exec off by default. |
| **Scheduled self-review** | Quarterly access + SSP/POA&M review and monthly audit-log review provide a recurring check (per the Continuous Monitoring Plan). |
| **External audit surface** | AWS-side logs and customer-visible results provide independent corroboration outside the operator's direct control. |

## 3. Residual risk & plan

The residual insider-misuse risk is assessed **Medium** (see Risk Assessment R11),
accepted for the current single-operator stage on the strength of the audit trail and
MFA. **On growth of the team, duties will be divided** — at minimum separating production
deploy authority from code authorship, and audit review from operations — and this memo
retired in favor of true separation of duties.

## 4. Determination

For the current environment, the compensating controls above provide **adequate
mitigation** for 3.1.4. This is documented, reviewed annually, and revisited immediately
upon adding personnel.

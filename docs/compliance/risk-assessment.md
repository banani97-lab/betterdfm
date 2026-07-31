# Risk Assessment — RapidDFM GovCloud

**System:** RapidDFM (GovCloud / ITAR-CUI edition)
**Owner:** Basel Anani · **Date:** 2026-07-31 · **Review:** annual + on material change
**Controls:** 3.11.1 (assess risk); informs 3.11.2/3.11.3, 3.12.x

> Initial risk assessment for the GovCloud boundary. Scope, method, and findings below.
> This is a point-in-time analysis; residual risks feed the POA&M.

---

## 1. Method

Qualitative assessment. For each identified risk: likelihood (Low/Med/High) × impact
(Low/Med/High) → risk rating, then existing controls and residual rating. Impact is
weighted for CUI/ITAR technical data (unauthorized foreign-person disclosure = high
impact by definition).

## 2. Assets

CUI/ITAR technical data (uploaded ODB++, parsed geometry, analysis results); user
credentials/identities; the availability of the analysis service; audit records.

## 3. Threats considered

Unauthorized access (external attacker, credential theft, foreign-person/deemed export);
insider misuse; data exfiltration; supply-chain (dependency/base-image compromise);
misconfiguration; denial of service; data loss.

## 4. Risk register

| # | Risk | Likelihood | Impact | Inherent | Existing controls | Residual |
|---|---|---|---|---|---|---|
| R1 | Credential theft → unauthorized CUI access | Med | High | High | MFA enforced (TOTP), strong password policy, short-lived JWTs, audit logging | **Low-Med** |
| R2 | Deemed export (foreign-person access) | Low | High | High | US-persons-only provisioning, GovCloud (US-persons ops), access review; provisioning attestation pending (POA&M-05) | **Med** |
| R3 | Data exfiltration to third party | Med | High | High | Third-party egress disabled (analytics/AI removed), FIPS-only AWS egress, org isolation | **Low** |
| R4 | CUI intercepted in transit | Low | High | Med | TLS 1.2/1.3 external, FIPS endpoints; **internal service TLS is a gap** (POA&M-01) | **Med** |
| R5 | Data at rest exposure | Low | High | High | SSE-KMS (CMK) on S3/RDS/SQS, private subnets, IAM | **Low** |
| R6 | Cross-tenant data leak | Low | High | High | Org-scoped authz enforced in code + tenant-isolation tests | **Low** |
| R7 | Supply-chain (dependency/image) compromise | Med | Med | Med | Maintained upstreams, immutable builds, patch SLA; automated scanning pending (POA&M-03) | **Med** |
| R8 | Malicious upload (parser exploit / zip-bomb) | Med | Med | Med | Hardened extraction (`filter="data"`), sized task memory; upload AV pending (POA&M-03) | **Low-Med** |
| R9 | Misconfiguration | Med | Med | Med | IaC baseline, least-functionality, CloudTrail change record | **Low-Med** |
| R10 | Data loss / availability | Low | Med | Med | Encrypted RDS backups (7d), versioned S3, IaC redeploy; RDS single-AZ | **Med** |
| R11 | Insider misuse (sole operator) | Low | High | Med | Full audit trail + MFA as compensating controls (documented, SoD memo) | **Med** |
| R12 | Lost admin MFA → lockout | Low | Med | Med | Backup-capable authenticator guidance; AWS-side MFA reset available to owner | **Low-Med** |

## 5. Key residual risks (feed POA&M)

- **R4 internal TLS** → POA&M-01
- **R2 provisioning-time US-persons attestation** → POA&M-05
- **R7 dependency/image scanning, R8 upload AV** → POA&M-03
- **R10 single-AZ RDS** → availability backlog (consider Multi-AZ for production)

## 6. Conclusion

Confidentiality risks — the ones that matter most for ITAR technical data — are reduced
to **Low / Low-Med** by the implemented controls (MFA, egress lockdown, encryption,
isolation). The notable residuals are internal-transit encryption (R4) and process/
tooling items, all tracked in the POA&M with owners and targets. No unmitigated High
residual risk remains.

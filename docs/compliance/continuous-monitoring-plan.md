# Continuous Monitoring Plan — RapidDFM GovCloud

**System:** RapidDFM (GovCloud / ITAR-CUI edition)
**Owner:** Basel Anani · **Effective:** 2026-07-31 · **Review:** annual
**Controls:** 3.12.3 (monitor controls on an ongoing basis); supports 3.3.x, 3.14.x

> Defines the ongoing activities that keep the control set effective between formal
> assessments, sized for a single-operator environment.

---

## 1. Monitoring tooling (in place)

| Source | Monitors | Retention |
|---|---|---|
| CloudTrail (multi-region, validated) | Control-plane / config / access | 365d CloudWatch + indefinite S3 |
| CloudWatch Logs (per service) | App behavior, auth, errors | 365d |
| VPC Flow Logs | Network flows at the boundary | 365d |
| ECS/ALB health + service events | Availability, deploys | — |
| Cognito | Auth events, MFA, lockouts | — |

## 2. Recurring activities & cadence

| Activity | Cadence | Owner |
|---|---|---|
| Audit-log review (auth anomalies, authz failures, admin actions) | **Monthly** + on any alert | Owner |
| User/role access review (remove stale accounts) | **Quarterly** | Owner |
| Dependency/base-image patch review → redeploy | **Monthly** + on critical advisory | Owner |
| Backup/restore spot-check (RDS) | **Quarterly** | Owner |
| SSP + POA&M review and update | **Quarterly** | Owner |
| Risk assessment refresh | **Annually** + on material change | Owner |
| IR tabletop test | **Annually** | Owner |
| Security awareness training | **Annually** + at onboarding | Owner |
| KMS key rotation verification | **Annually** | Owner |

## 3. Change-driven monitoring

Any change touching authentication, cryptography, boundary/networking, logging, or CUI
handling triggers an immediate security-impact review and an SSP update (per the
Configuration & Change Management Policy).

## 4. Metrics tracked

Failed-authentication rate, authorization-failure count, unresolved
critical/high vulnerabilities and age, open POA&M items and overdue targets, days since
last audit review, backup success.

## 5. Reporting

Findings and status are recorded in the SSP/POA&M at each quarterly review. Material
issues are handled immediately via the Incident Response Plan or a new POA&M entry.

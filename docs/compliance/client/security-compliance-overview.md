# RapidDFM — Security & Compliance Overview

**Prepared for:** AbsoluteEMS
**Prepared by:** Saturn Solutions / RapidDFM · Basel Anani, System Owner
**Date:** 2026-07-31 · **Classification:** For AbsoluteEMS evaluation use

> This overview summarizes the security architecture and compliance posture of the
> RapidDFM GovCloud environment. It is a factual statement of controls in place and work
> in progress; it is **not** a legal certification of ITAR compliance and is **not** legal
> advice. Formal export-control determinations are made by qualified counsel.

---

## 1. Summary

RapidDFM operates a **dedicated AWS GovCloud (US) environment purpose-built to handle
export-controlled (ITAR) and CUI technical data**. It runs entirely within a US-persons
boundary, separate from our commercial platform, and implements the security controls
expected of a system handling controlled defense technical data — aligned to **NIST SP
800-171 Rev. 2**, the standard that underpins DFARS 252.204-7012 and CMMC.

**Readiness at a glance:**

- **Non-ITAR / commercial designs: ready for use today.**
- **ITAR / export-controlled designs: environment is built and controlled for it**, with a
  short set of formal completion items in progress (§5).

---

## 2. Environment & isolation

- **AWS GovCloud (US)**, region us-gov-west-1 — a US-persons-operated, FedRAMP High
  authorized region designed for controlled workloads.
- A **dedicated account and boundary**, physically and logically separate from our
  commercial edition. No shared data plane.
- **Invite-only, US-persons access.** No public sign-up. Each user is provisioned by an
  administrator who must attest the user is a U.S. person before the account is created.

## 3. Security controls in place (verified)

| Area | Control |
|---|---|
| **Authentication** | **Multi-factor authentication enforced for every user** (TOTP), including administrators. |
| **Authorization** | Role-based access + strict per-organization data isolation (verified by automated tests: one organization cannot access another's data). |
| **Encryption in transit** | TLS 1.2/1.3 externally; **FIPS 140-validated endpoints** for all AWS service calls; internal service-to-service traffic is encrypted (TLS). |
| **Encryption at rest** | All data (uploads, database, queues, backups) encrypted with a customer-managed **FIPS 140-validated KMS key**. |
| **Audit** | Multi-region audit logging with log-file integrity validation, 365-day retention, and automated security alerting. |
| **Network boundary** | Virtual private cloud with private subnets and default-deny security groups; databases and compute have no public exposure. |
| **Data-egress control** | No third-party analytics or external AI services in the environment — controlled data does not leave the boundary. |
| **Upload handling** | Design-file extraction hardened against malformed/malicious archives. |

## 4. Compliance framework

- **NIST SP 800-171 Rev. 2** — the 110-control standard for protecting Controlled
  Unclassified Information.
- **102 of 110 controls are fully addressed** (implemented or inherited from AWS
  GovCloud). The remainder are tracked in a formal **Plan of Action & Milestones (POA&M)**
  with owners and target dates — the standard, expected way to operate.
- A documented **System Security Plan**, security policies, incident-response plan (with
  DoD 72-hour reporting procedures), risk assessment, and continuous-monitoring plan are
  maintained.
- **Self-assessed SPRS score: approximately 96 / 110** (pending formal submission).

## 5. In progress (full transparency)

To reach a fully formalized ITAR posture, the following are underway:

1. **Export-control counsel determination** — a written scoping/registration determination
   from qualified ITAR counsel.
2. **SPRS submission** — filing the self-assessment score to the DoD SPRS system.
3. **One remaining technical POA&M item** — automated malware scanning of uploaded files.

None of these affect the handling of **non-ITAR** designs, which the environment is ready
to process now.

## 6. Shared responsibility

RapidDFM provides the secured environment and controls above. Access authorization
(ensuring only US persons are granted accounts) is a shared responsibility exercised at
user provisioning. AbsoluteEMS remains responsible for its own export-control obligations
and internal handling of controlled data.

## 7. Contact

Questions or a deeper security review (SSP, POA&M, control evidence) can be directed to
Basel Anani, System Owner, Saturn Solutions.

---

*This document describes the system's security posture as of the date above. It is not a
certification, attestation of ITAR compliance, or legal advice. Controlled-data handling
should proceed consistent with each party's export-control determinations.*

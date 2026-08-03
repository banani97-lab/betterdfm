# Data Protection Summary — RapidDFM GovCloud

**Provider:** Saturn Solutions / RapidDFM · **Prepared for:** AbsoluteEMS · **Date:** 2026-07-31

How your design data is protected throughout its lifecycle in the RapidDFM GovCloud
environment.

## Data residency
- Hosted entirely in **AWS GovCloud (US)**, `us-gov-west-1` — a US-persons-operated,
  FedRAMP High authorized region. Data does not leave the US boundary.
- A **dedicated account**, isolated from our commercial platform.

## Encryption
- **At rest:** all data — uploaded design files, database, job queue, backups — is
  encrypted with a customer-managed, **FIPS 140-validated** key (AWS KMS).
- **In transit:** TLS 1.2/1.3 for all external connections; **FIPS-validated endpoints**
  for internal AWS calls; internal service-to-service traffic is TLS-encrypted.

## Access
- **US persons only.** Invite-only; no public sign-up. Each account is provisioned by an
  administrator who attests the user is a U.S. person.
- **Multi-factor authentication is mandatory** for every user, including administrators.
- **Least privilege + tenant isolation:** role-based access, and each organization's data
  is strictly segregated — verified by automated tests proving one organization cannot
  read another's data.

## Data handling & isolation
- Your uploads and their analysis results are scoped to your organization only.
- **No third-party data egress:** the environment contains no external analytics or AI
  services — your controlled data is not sent outside the boundary.

## Subprocessors
- **AWS GovCloud (US)** is the sole infrastructure subprocessor. No commercial analytics,
  AI, email-marketing, or tracking services process your data.

## Retention & deletion
- **Audit logs:** retained 365 days (online) with long-term archival.
- **Backups:** encrypted, automated database backups (7-day point-in-time).
- **Deletion:** on request, a submission and all its derived data (analysis, results,
  stored files, and all storage versions) are permanently removed — a process we have
  performed and verified.

## Availability & recovery
- Infrastructure is defined as code and reproducible; encrypted backups support recovery.

## Monitoring & incident response
- Multi-region audit logging with automated security alerting and an audit-failure alarm.
- A documented incident-response plan, including US-government (DoD) 72-hour reporting
  procedures for controlled-data incidents.

## Compliance alignment
- Controls aligned to **NIST SP 800-171 Rev. 2** (the CUI protection standard). Current
  self-assessment: **104 of 110 controls addressed**, with a formal Plan of Action &
  Milestones for the remainder. A deeper review (SSP / control evidence) is available to
  qualified reviewers under NDA.

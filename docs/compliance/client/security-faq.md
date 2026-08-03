# Security FAQ — RapidDFM GovCloud

**Provider:** Saturn Solutions / RapidDFM · **Prepared for:** AbsoluteEMS · **Date:** 2026-07-31

Answers to common security-review questions. For deeper detail (System Security Plan,
control evidence), a review can be arranged under NDA.

**Where is our data hosted?**
AWS GovCloud (US), region us-gov-west-1 — a US-persons-operated, FedRAMP High authorized
region. Data stays within the US boundary and in a dedicated account separate from our
commercial platform.

**Is our data encrypted?**
Yes. At rest with a FIPS 140-validated customer-managed key (KMS); in transit with TLS
1.2/1.3 and FIPS-validated endpoints, including internal service-to-service traffic.

**Who can access our data?**
Only US persons you authorize. Access is invite-only, MFA is mandatory for all users, and
each organization's data is strictly isolated (verified by automated tests). Your admins
control your users; we provision only after a US-person attestation.

**Do you use any third-party services or subprocessors?**
AWS GovCloud (US) is the only infrastructure subprocessor. There are no commercial
analytics, AI, tracking, or marketing services processing your data — controlled data does
not leave the boundary.

**Is multi-factor authentication required?**
Yes — enforced for every user, including administrators, via time-based authenticator apps.

**What compliance framework do you follow?**
NIST SP 800-171 Rev. 2 (the standard for protecting Controlled Unclassified Information),
implemented in a GovCloud boundary. Current self-assessment: 104 of 110 controls addressed
with a formal Plan of Action & Milestones for the remainder.

**Are you ITAR compliant / ITAR registered?**
The environment is **purpose-built to handle ITAR/export-controlled and CUI technical
data** — a GovCloud US-persons boundary with the controls above. It is **ready for
non-ITAR designs today.** Formal ITAR items (an export-control counsel determination and
SPRS submission) are in progress; we are transparent about that rather than overstating a
legal conclusion. We will confirm status before controlled designs are processed.

**How do you handle a security incident?**
We maintain a documented incident-response plan covering detection, containment,
eradication, recovery, and reporting — including US-government (DoD) 72-hour reporting
procedures for controlled-data incidents. Affected customers are notified.

**Can we get our data deleted?**
Yes. On request, a submission and all derived data (analysis, results, stored files, and
all storage versions) are permanently removed — a process we have performed and verified.

**Do you have backups / disaster recovery?**
Yes — encrypted automated database backups with point-in-time recovery, and
infrastructure defined as code for reproducible recovery.

**Have you had a third-party security assessment?**
We maintain a self-assessment against NIST 800-171 (estimated SPRS ~98/110) with a
documented SSP and POA&M. A formal third-party (e.g. CMMC) assessment is a future step;
control evidence is available for review under NDA.

**Who do we contact with security questions?**
Basel Anani, System Owner, Saturn Solutions.

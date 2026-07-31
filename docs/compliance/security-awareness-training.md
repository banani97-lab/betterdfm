# Security Awareness Training — RapidDFM GovCloud

**System:** RapidDFM (GovCloud / ITAR-CUI edition)
**Owner:** Basel Anani · **Effective:** 2026-07-31 · **Cadence:** at onboarding + annually
**Controls:** 3.2.1 (awareness), 3.2.2 (role-based), 3.2.3 (insider threat)

> Applies to everyone with access to the boundary. For the current single-operator
> environment, the System Owner completes and records this training; the same module is
> assigned to any future personnel at onboarding.

---

## 1. Awareness topics (all personnel)

- **What CUI/ITAR technical data is** and why it matters: PCB design files here are
  export-controlled; disclosure to a foreign person — even domestically — can be an ITAR
  violation (deemed export).
- **US-persons rule:** only US persons may access the system or its data. Never grant
  access, share credentials, or forward CUI to anyone whose US-person status is unverified.
- **Handling CUI:** keep it inside the boundary; do not copy CUI to personal devices,
  email, chat, or non-approved cloud services; no third-party tools with the data.
- **Authentication hygiene:** protect your password; MFA is mandatory; use a backup-capable
  authenticator; never share TOTP codes; report lost devices immediately.
- **Phishing / social engineering:** verify unexpected requests; do not act on emailed
  links or credential prompts related to the system without verification.
- **Incident reporting:** if you suspect unauthorized access, data exposure, or a lost
  credential/device, invoke the Incident Response Plan **immediately** (CUI incidents have
  a 72-hour DoD reporting clock).

## 2. Role-based topics

- **Administrators:** privileged-access responsibilities; provisioning only verified
  US persons; least-privilege role assignment; quarterly access reviews; MFA on the admin
  console is non-negotiable.
- **Developers / operators:** secure change management (IaC, reviewed commits); never
  weaken egress/crypto/auth controls without a security-impact review and POA&M update;
  never introduce third-party data egress in the gov build.

## 3. Insider-threat awareness

Recognize and report indicators: attempts to access data outside one's need-to-know,
unusual data movement, requests to bypass controls, or pressure to grant access to
unverified persons. In a small team the compensating control is a **complete, reviewed
audit trail** — every action is attributable and logged.

## 4. Completion record

| Name | Role | Date completed | Method |
|---|---|---|---|
| Basel Anani | System Owner / Admin | 2026-07-31 | Self-study of this module |
| _[future personnel]_ | | | at onboarding |

Records are retained for the life of the person's access + audit-retention period.

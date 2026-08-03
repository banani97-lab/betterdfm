# Client Security Package — RapidDFM GovCloud

This folder contains the **client-facing** security documents — curated and safe to share
with a customer (e.g. AbsoluteEMS). Everything here is accurate and high-level; none of it
exposes internal weaknesses.

## Send these

| Document | Purpose |
|---|---|
| `security-compliance-overview.md` | The anchor: environment, controls, compliance posture, ITAR readiness. Start here. |
| `data-protection-summary.md` | How customer data is encrypted, isolated, retained, deleted. |
| `shared-responsibility-matrix.md` | Who owns what (AWS / RapidDFM / customer). |
| `security-faq.md` | Answers to common security-review questions, incl. the honest ITAR answer. |

## Do NOT send (internal only)

These live in `docs/compliance/` and are for internal use or NDA-gated formal assessment
only — they detail your posture **and your gaps**:

- `SSP.md` — full System Security Plan
- `nist-800-171-self-assessment.md` — score + open-control deductions
- **`(POA&M is inside the SSP)`** — your list of open weaknesses/target dates
- `risk-assessment.md` — your risk register
- `separation-of-duties-memo.md` — reveals single-operator staffing
- `incident-response-plan.md`, `continuous-monitoring-plan.md`,
  `information-security-policy.md`, `security-awareness-training.md`,
  `ir-tabletop-2026-07.md` — internal procedures

If a prime or the customer's security team formally requests the SSP or control evidence,
provide it **under NDA**, not as a routine attachment.

## Before sending

- Confirm the ITAR-readiness framing matches your current status (the FAQ and overview are
  written to be honest, not to overstate — keep them that way).
- These are drafts pending your (ISSO/owner) review; the numbers (104/110, ~98 SPRS)
  reflect a self-assessment as of the date on each document.

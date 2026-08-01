# NIST SP 800-171 Self-Assessment & SPRS Score

**System:** RapidDFM (GovCloud / ITAR-CUI edition)
**Assessor:** Basel Anani (System Owner) · **Date:** 2026-07-31
**Basis:** SSP (`SSP.md`) + supporting policies/procedures · **Method:** DoD Assessment
Methodology (SPRS scoring)
**Control:** 3.12.1 (assess security controls)

> **Submission is yours to do.** This is the *self-assessment* that supports an SPRS
> entry. Submitting the score to SPRS requires your CAGE code and login — do that step
> yourself. **Confirm the exact point deductions against the current official DoD SPRS
> scoring template before submitting;** the weights below reflect the published 1/3/5
> methodology but should be re-verified.

---

## 1. Methodology (summary)

The DoD Assessment Methodology starts at **110** and deducts **1, 3, or 5 points** for
each of the 110 controls **not fully implemented**, weighted by security impact. Controls
on a POA&M still deduct until closed. A few controls allow partial credit; most are
all-or-nothing.

## 2. Result

- **Controls fully implemented or inherited/N/A: 102 of 110**
- **Open (not yet fully implemented): 8** — all tracked in the SSP POA&M.
  *(3.1.9 login banner and 3.9.1 US-persons attestation closed 2026-07-31 with POA&M-05.)*

## 3. Deductions (open controls)

| Control | Title | Est. weight | On POA&M | Note |
|---|---|---|---|---|
| 3.1.3 | Control CUI flow | 1 | 01 | internal TLS |
| 3.1.5 | Least privilege | 3 | 02 | scoped deploy role (deferred) |
| ~~3.1.9~~ | System-use notification | — | ✅ closed | login banner deployed |
| 3.1.10 | Session lock | 1 | — | UI inactivity lock |
| 3.5.6 | Disable inactive identifiers | 1 | — | automate disablement |
| ~~3.9.1~~ | Screen personnel | — | ✅ closed | US-persons attestation deployed |
| 3.13.8 | Encrypt CUI in transit | 1 | 01 | internal TLS |
| 3.14.2 | Malicious-code protection | 5 | 03 | upload AV (extraction hardened; partial) |
| 3.14.4 | Update malicious-code protection | 1 | 03 | tied to 3.14.2 |
| 3.14.5 | Periodic + real-time scans | 1 | 03 | dep + image scan in place; upload real-time pending |

**Estimated deduction: ~14 points → estimated SPRS score ≈ 96 / 110** (after POA&M-05
closed 3.1.9 + 3.9.1; pending official-template verification).

## 4. Interpretation

An estimated **~92/110** is a strong score for a system operating under a POA&M. The
implemented set includes the high-weight controls (access control, MFA [3.5.3=5],
FIPS crypto [3.13.11=5], encryption at rest [3.13.16], audit, baseline/config [3.4.1/2=5]).
The largest single remaining deduction is **3.14.2 (malicious-code / upload AV, 5 pts)** —
the highest-value next target. Closing the current POA&M raises the score toward **110**:

| If you close… | Score moves toward |
|---|---|
| POA&M-05 (banner + attestation) | +~4 (3.1.9, 3.9.1) |
| POA&M-03 upload AV | +~7 (3.14.2, 3.14.4, 3.14.5) |
| POA&M-01 internal TLS | +~2 (3.1.3, 3.13.8) |
| POA&M-02 scoped deploy | +~3 (3.1.5) |

## 5. Attestation

I assessed the RapidDFM GovCloud system against NIST SP 800-171 Rev. 2 using the DoD
Assessment Methodology. The results above reflect the system state on the assessment
date. Open items are documented with remediation plans in the POA&M.

Assessor: Basel Anani · Date: 2026-07-31 · (Signature on file for submission)

# Incident Response Plan — RapidDFM GovCloud

**System:** RapidDFM (GovCloud / ITAR-CUI edition)
**Owner:** Basel Anani (System Owner / Incident Handler)
**Effective:** 2026-07-31 · **Review:** annual + after any incident
**Controls:** 3.6.1, 3.6.2 (test: 3.6.3 — scheduled, see §7)

> Sized for a single-operator environment. The System Owner is the primary incident
> handler; escalation paths (counsel, AWS support, customers, government) are below.

---

## 1. Purpose & scope

Establishes how security incidents affecting the RapidDFM GovCloud boundary — especially
those involving CUI / export-controlled technical data — are detected, contained,
eradicated, recovered, reported, and reviewed.

## 2. Definitions

- **Event:** any observable occurrence in the system.
- **Incident:** an event that actually or imminently jeopardizes the confidentiality,
  integrity, or availability of CUI or the system.
- **CUI incident:** any incident with suspected or confirmed unauthorized access to, or
  exfiltration of, CUI/ITAR technical data. Triggers external reporting (§5).

## 3. Roles

| Role | Who | Responsibility |
|---|---|---|
| Incident Handler | System Owner (Basel Anani) | Detection→recovery, decisions, records |
| Legal / export counsel | [contact] | ITAR/CUI reporting determinations |
| Cloud support | AWS GovCloud Support | Infrastructure containment assistance |
| Affected customers | Org admins | Notification when their data is involved |

## 4. Detection sources

CloudWatch alarms, CloudTrail, VPC flow logs, application error logs, AWS
GuardDuty/Security findings (if enabled), customer reports. Authentication anomalies and
authorization-failure spikes are primary indicators.

## 5. Response procedure

1. **Detect & record.** Open an incident record (timestamp, source, description). Assign
   a severity (CUI incident = high by default).
2. **Contain.** Limit damage: revoke/rotate affected credentials and Cognito sessions;
   tighten security groups; disable affected users; if needed, take a service offline.
   Preserve evidence (snapshots, log exports) before altering systems.
3. **Assess CUI impact.** Determine whether CUI was accessed or exfiltrated. **If yes,
   this is a CUI incident — proceed to reporting (§6) without delay.**
4. **Eradicate.** Remove the cause (patch, rebuild image, revoke access, close the gap).
5. **Recover.** Restore from known-good IaC/images/backups; verify integrity; monitor
   closely post-recovery.
6. **Document & review.** Complete the incident record and conduct a post-incident review
   (§7).

## 6. External reporting (CUI incidents)

- **DoD / DFARS:** For incidents involving covered defense information, report to DoD via
  **dibnet.dod.mil within 72 hours** of discovery, per DFARS 252.204-7012. A medium-
  assurance certificate is required; obtain/maintain in advance.
- **Export control:** Consult export counsel immediately on any suspected unauthorized
  foreign-person access (potential ITAR violation / voluntary disclosure considerations).
- **Customers:** Notify affected organizations' admins per contractual obligations.
- **AWS:** Engage AWS support for infrastructure-level incidents.

Reporting decisions and timestamps are captured in the incident record.

## 7. Testing & maintenance

- **Test:** conduct a **tabletop exercise annually** walking a simulated CUI-exfiltration
  scenario end-to-end; record results and remediation actions. (First test: POA&M-04.)
- **Maintenance:** review and update this plan annually and after every incident.

## 8. Incident record template

```
Incident ID / date-time (UTC):
Detected by / source:
Severity / CUI incident? (Y/N):
Description:
Systems / data affected:
Containment actions + times:
CUI impact assessment:
External reports filed (who / when):
Eradication + recovery actions:
Root cause:
Post-incident improvements + POA&M updates:
```

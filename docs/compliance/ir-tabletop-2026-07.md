# Incident Response Tabletop Exercise — Record

**System:** RapidDFM (GovCloud / ITAR-CUI edition)
**Date:** 2026-07-31 · **Facilitator/Participant:** Basel Anani (System Owner / Incident Handler)
**Control:** 3.6.3 (test the incident-response capability) · **Plan tested:** `incident-response-plan.md`
**Type:** Discussion-based tabletop (single-operator)

---

## 1. Objective

Walk the Incident Response Plan end-to-end against a realistic CUI-exfiltration
scenario to validate the procedure, the roles, the reporting obligations, and the
containment/recovery steps, and to surface gaps.

## 2. Scenario

> A customer (org admin) reports that a set of their uploaded ODB++ designs appear in a
> results view that is not theirs. Simultaneously, CloudWatch shows a spike in
> authorization-failure events from one user account, and CloudTrail shows that account
> enumerating S3 objects under another org's `submissions/` prefix. Suspected
> cross-tenant access to export-controlled technical data.

## 3. Walkthrough (injects → response)

| # | Inject | Response taken (per plan) |
|---|---|---|
| 1 | Customer report + alarm fires | Open incident record (UTC time, source). Severity = **High / CUI incident** (potential unauthorized access to ITAR data). |
| 2 | Identify the account | From CloudTrail/app logs, identify the Cognito sub + org. **Contain:** disable the user in Cognito + DB; revoke sessions; rotate any exposed credentials. |
| 3 | Determine scope | Query audit logs for what objects/records the account actually accessed. Preserve evidence (export relevant log ranges, snapshot state) **before** further changes. |
| 4 | Assess CUI impact | Confirm whether cross-org CUI was actually read. Tenant-isolation authz tests exist; verify whether they were bypassed or a data-scoping bug was involved. |
| 5 | CUI incident confirmed | Trigger external reporting (§6 of the plan): DoD via **dibnet.dod.mil within 72 hours**; notify **export counsel**; notify the **affected customer**(s). Record timestamps. |
| 6 | Eradicate | If a code/authorization defect enabled it, patch + rebuild + redeploy; if credential compromise, complete credential rotation. |
| 7 | Recover | Verify org-scoping restored; monitor closely; confirm no residual access. |
| 8 | Review | Complete incident record; root-cause; add POA&M items for any control gap found. |

## 4. Findings

| # | Finding | Action |
|---|---|---|
| F1 | The 72-hour DoD reporting clock requires a **medium-assurance certificate** for dibnet — must be obtained **in advance**, not during an incident. | Add "obtain dibnet reporting certificate" as a pre-incident readiness task. |
| F2 | Export-counsel contact is a `[contact]` placeholder in the IR plan. | Fill in a named counsel contact before go-live with real customers. |
| F3 | Evidence-preservation step (log export/snapshot) should be a concrete, rehearsed command set. | Add a short evidence-capture checklist to the IR plan. |
| F4 | Single operator = no independent second reviewer during an incident. | Accepted; compensating control is the complete audit trail (SoD memo). Revisit on team growth. |

## 5. Outcome

The IR Plan is **usable and sufficient** for the scenario. No blocking gaps; four
improvements captured (F1–F3 actionable, F4 accepted). Next tabletop: within 12 months
or after any real incident.

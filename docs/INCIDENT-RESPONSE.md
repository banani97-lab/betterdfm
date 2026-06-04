# Incident Response Runbook (DFARS 252.204-7012)

Scaffolding for the cyber-incident reporting obligation that flows down with
CUI. See [`ITAR-GOVCLOUD-MIGRATION.md`](ITAR-GOVCLOUD-MIGRATION.md) for the
broader compliance plan. This is a process stub to be fleshed out before real
CUI is accepted (Phase 8); the technical detection plumbing is in
`infra/terraform/cloudtrail.tf` and `monitoring.tf`.

## The obligation

DFARS 252.204-7012 (c)-(g): when a cyber incident affects a covered contractor
information system or the CUI on it, the contractor must **report to DoD within
72 hours of discovery** via DIBNet (https://dibnet.dod.mil, requires a
medium-assurance DoD-approved certificate), preserve images and relevant
monitoring data for **at least 90 days**, and support DoD damage assessment.

As a CSP handling CUI for our customers, we must also notify affected
customers so they can meet their own flow-down obligations.

## Detection (what is wired)

- **CloudTrail** records management events account-wide plus S3 data events on
  the uploads bucket, to a CMK-encrypted, versioned, log-file-validated bucket
  and to CloudWatch Logs.
- **Metric-filter alarms** (`monitoring.tf`) fire on root-account usage,
  unauthorized/AccessDenied API calls, and console sign-in without MFA, to the
  `*-security-alerts` SNS topic. Subscribe responders to that topic.
- **VPC flow logs** capture network metadata.

## Response steps (fill in owners + contacts before go-live)

1. **Detect / triage.** Alarm or report received. Assign an incident lead.
   Classify severity and whether CUI is in scope.
2. **Contain.** Revoke credentials/sessions, isolate affected tasks/subnets,
   rotate the DB secret and CMK grants as needed.
3. **Preserve.** Snapshot affected resources; export the relevant CloudTrail /
   CloudWatch / flow-log ranges. Retain >= 90 days.
4. **Report.** If CUI is affected, submit to DIBNet within **72 hours** of
   discovery. Record the incident report number.
5. **Notify customers** whose CUI may be affected.
6. **Recover + postmortem.** Restore from known-good state; write a blameless
   postmortem; track remediation in the POA&M.

## TODO before accepting CUI

- [ ] Obtain the DoD medium-assurance certificate for DIBNet.
- [ ] Name the incident lead + 24/7 contact path; subscribe them to the SNS topic.
- [ ] Define customer-notification templates + SLAs.
- [ ] Tabletop-exercise this runbook.

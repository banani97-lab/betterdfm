# Shared Responsibility Matrix — RapidDFM GovCloud

**Provider:** Saturn Solutions / RapidDFM · **Prepared for:** AbsoluteEMS · **Date:** 2026-07-31

Security in the RapidDFM GovCloud environment is a shared responsibility across three
parties. This matrix clarifies who owns what.

| Responsibility | AWS (GovCloud) | RapidDFM (Saturn Solutions) | AbsoluteEMS (Customer) |
|---|---|---|---|
| Physical & environmental security of data centers | ✅ Owns | — | — |
| Hardware, hypervisor, network infrastructure | ✅ Owns | — | — |
| US-persons-operated cloud region | ✅ Owns | Selects & configures | — |
| Application security (authn, authz, isolation) | — | ✅ Owns | — |
| Multi-factor authentication (enforced) | — | ✅ Provides | Users enroll on first login |
| Encryption at rest & in transit | Provides KMS/HSM | ✅ Configures & enforces | — |
| Audit logging & monitoring | Provides services | ✅ Configures & reviews | — |
| Network boundary (VPC, security groups) | Provides | ✅ Configures | — |
| User provisioning & US-person verification | — | Enforces attestation at creation | ✅ **Attests** each user is a U.S. person |
| Account & credential hygiene (per user) | — | Policy + MFA | ✅ Protects credentials, uses backup-capable authenticator |
| Data classification of uploaded designs | — | Treats boundary as CUI | ✅ **Determines** what is ITAR/export-controlled |
| Export-control compliance for their designs | — | Provides controlled environment | ✅ **Owns** their ITAR/EAR obligations |
| Incident detection & response (platform) | Provides signals | ✅ Owns IR process + reporting | Reports suspected issues |
| Incident reporting for their controlled data | — | Notifies customer | ✅ Owns their downstream reporting |
| Data retention & deletion requests | Executes deletes | ✅ Provides + executes | ✅ Requests |

**Key point for AbsoluteEMS:** you retain responsibility for (1) attesting that each user
you have us provision is a U.S. person, (2) classifying which of your designs are
export-controlled, and (3) your own downstream export-control obligations. RapidDFM
provides the secured, US-persons GovCloud environment and controls to hold that data.

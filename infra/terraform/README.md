# RapidDFM GovCloud Infrastructure (Terraform)

Infrastructure-as-code for the ITAR/CUI GovCloud boundary. See
[`../../docs/ITAR-GOVCLOUD-MIGRATION.md`](../../docs/ITAR-GOVCLOUD-MIGRATION.md)
for the full plan and decisions.

All resources deploy to a GovCloud region (`us-gov-west-1`, partition
`aws-us-gov`). Apply with US-person credentials only.

## What WS0 provisions (this commit)

- **VPC** with public + private subnets across 2 AZs, IGW, NAT, and **VPC flow
  logs** (encrypted with the CMK). Workloads run in private subnets; only the
  ALB/NAT touch public subnets.
- **Customer-managed KMS CMK** (`alias/rapiddfm-cmk`) with key rotation and a
  US-person-scoped policy. This is the key behind the 22 CFR 120.54 encryption
  position; it encrypts S3, RDS, CloudWatch Logs, and SQS in later workstreams.
- **GitHub Actions OIDC provider + deploy role** (trust only) for keyless CI,
  replacing the static AWS keys in the deploy workflow.

Still to come: WS2 (ECS/Fargate for web+api+worker+gerbonara, ALB, RDS, S3,
SQS, Cognito), WS4 (encryption enforcement), WS6 (CloudTrail + central logs).

## Usage

The GovCloud account starts empty, so first bootstrap a remote state backend
(an encrypted S3 bucket; S3 native locking means no DynamoDB needed), then run
the root module against it.

```bash
# 0. Bootstrap the state backend (LOCAL state, run once).
cd infra/terraform/bootstrap
terraform init
terraform apply                      # creates the state bucket + its CMK
terraform output backend_hcl         # copy these lines...

# 1. ...into the root module's backend config.
cd ..
cp backend.hcl.example backend.hcl   # paste the bootstrap output values

# 2. Set inputs. domain_name is pre-set; github_org is OPTIONAL (leave empty to
#    skip the CI deploy role - manual deploys do not need it).
cp terraform.tfvars.example terraform.tfvars   # edit

# 3. Init against the remote backend, then review + apply.
terraform init -backend-config=backend.hcl
terraform plan
terraform apply
```

Run everything with US-person GovCloud credentials. `backend.hcl` and
`terraform.tfvars` are gitignored; never commit real values. The bootstrap
module keeps LOCAL state (it creates the backend the root module uses) - back
that state up out of band.

## Key outputs

`kms_key_arn`, `vpc_id`, `private_subnet_ids`, `github_deploy_role_arn` are
consumed by later workstreams (and the deploy workflow). Run `terraform output`
after apply.

## Notes on US-person enforcement

IAM principals are not US-person-aware on their own. Key administrators, the
deploy role's human approvers, and anyone with console/API access to this
account must be US persons; that is enforced via IAM group membership +
personnel process (PS/AC controls), not by Terraform.

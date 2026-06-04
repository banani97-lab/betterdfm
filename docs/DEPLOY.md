# GovCloud Deploy (Safe Parallel Bring-Up)

How to stand up the GovCloud environment **alongside** the existing
Vercel/commercial production without touching it, and how to tear it down. See
[`ITAR-GOVCLOUD-MIGRATION.md`](ITAR-GOVCLOUD-MIGRATION.md) for the why.

## Safety model (read first)

- **Production is not touched by any step here.** GovCloud is a separate AWS
  account/partition (`aws-us-gov`). A gov `apply`/`destroy` cannot see or change
  commercial resources.
- **Do NOT merge this branch to `main` yet.** Merging is the only action with
  commercial impact (it triggers `deploy.yml` + a Vercel rebuild). The first gov
  bring-up below is done out-of-band, so `main` and Vercel stay untouched.
- The existing domain keeps pointing at Vercel. Gov uses a **subdomain**
  (`gov.rapiddfm.com`) with its own DNS records, added alongside the existing
  ones. Nothing existing is edited.
- Everything is reversible: `terraform destroy` + remove the gov DNS records.

## Prerequisites

- A GovCloud account, with **US-person** credentials configured for the CLI.
- `terraform` >= 1.10, `docker`, `aws` CLI.
- DNS control for `rapiddfm.com` (to add the `gov.` subdomain + ACM validation).
  The apex/`www` keep pointing at Vercel; you only add `gov.*` records.

## 1. Bootstrap the state backend (once)

```bash
cd infra/terraform/bootstrap
terraform init
terraform apply
terraform output backend_hcl     # copy these lines
```

## 2. Configure the root module

```bash
cd ..
cp backend.hcl.example backend.hcl          # paste the bootstrap output
cp terraform.tfvars.example terraform.tfvars
# In terraform.tfvars set:
#   domain_name = "gov.rapiddfm.com"
#   github_org  = "<your-github-org>"
terraform init -backend-config=backend.hcl
```

## 3. Issue the ACM cert (two-step, so apply does not block on DNS)

```bash
# a. Create just the cert.
terraform apply -target='aws_acm_certificate.main[0]'
# b. Get the DNS validation record and create it in your DNS provider.
terraform output acm_certificate_validation
# c. Wait until the cert shows ISSUED (a few minutes):
aws acm list-certificates --region us-gov-west-1
```

## 4. Apply everything

```bash
terraform apply        # ~15 min (RDS). Creates VPC, RDS, Cognito, ECR, ECS, ALB, CloudTrail.
terraform output       # note ecr_repository_urls, api_url, cognito_*_client_id, ecs_cluster
```

ECS services come up referencing images that do not exist yet, so tasks won't
be healthy until the first image push (next step).

## 5. First image push (out-of-band, no merge)

Done manually with your gov credentials so `main`/CI is never involved. The web
image bakes its public config at build time, so pass the build args from the
`terraform output` values.

```bash
ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
ECR=$ACCOUNT.dkr.ecr.us-gov-west-1.amazonaws.com
aws ecr get-login-password --region us-gov-west-1 | docker login --username AWS --password-stdin $ECR

# web (build args from terraform output: api_url, cognito_app_client_id, cognito_admin_client_id)
docker build -f apps/web/Dockerfile apps/web \
  --build-arg NEXT_PUBLIC_API_URL=https://api.gov.rapiddfm.com \
  --build-arg NEXT_PUBLIC_COGNITO_REGION=us-gov-west-1 \
  --build-arg NEXT_PUBLIC_COGNITO_CLIENT_ID=<cognito_app_client_id> \
  --build-arg NEXT_PUBLIC_ADMIN_COGNITO_CLIENT_ID=<cognito_admin_client_id> \
  -t $ECR/rapiddfm-web:latest
docker build -f apps/api/Dockerfile . -t $ECR/rapiddfm-api:latest
docker build -f workers/dfm-worker/Dockerfile . -t $ECR/rapiddfm-worker:latest
docker build -f sidecar/gerbonara/Dockerfile sidecar/gerbonara -t $ECR/rapiddfm-gerbonara:latest

for s in web api worker gerbonara; do docker push $ECR/rapiddfm-$s:latest; done

# Roll the services onto the freshly pushed :latest images.
for s in web api worker gerbonara; do
  aws ecs update-service --cluster rapiddfm --service rapiddfm-$s \
    --force-new-deployment --region us-gov-west-1 >/dev/null
done
```

## 6. DNS for the app

Point the gov subdomains at the ALB (`terraform output alb_dns_name`):
`app.gov.rapiddfm.com` and `api.gov.rapiddfm.com` -> ALB (CNAME/ALIAS).

## 7. Seed the first admin

No self-signup, and provisioning needs an existing admin, so create the first
admin user once in the Cognito console (a US person): add a user to the pool,
set `custom:role=ADMIN` and `custom:orgId`, associate the admin app client. That
admin can then provision everyone else via the admin route.

## 8. Smoke test

- `https://api.gov.rapiddfm.com/health` -> 200
- `https://app.gov.rapiddfm.com` loads
- Log in (USER_PASSWORD_AUTH), upload a **non-CUI** board, confirm analysis.
- Confirm CloudTrail + flow logs are flowing.

## Rollback (total, prod unaffected)

```bash
cd infra/terraform
terraform destroy            # tears down the gov env only
# then remove the gov.* DNS records
```

Commercial/Vercel was never touched, so there is nothing to restore there.

## Later: enabling CI + the real cutover

- To use `deploy-govcloud.yml` (instead of manual pushes), it must exist on the
  default branch, so this branch needs to merge first. Before merging, make the
  commercial side safe: set `NON_CUI_ALPHA_MODE=false` on the commercial API
  (otherwise the upload guardrail turns on there too), and confirm commercial
  has a real `JWT_ISSUER` (the dev bypass is compiled out of prod builds). Then
  set the GitHub repo variables from `terraform output` (see the workflow header).
- The real cutover (point the production domain at gov, retire Vercel) is a
  separate, deliberate DNS change made only after gov is proven, and reversible
  by pointing DNS back at Vercel.

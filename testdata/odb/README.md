# ODB++ Test Fixtures

Real ODB++ `.tgz` files are stored in S3 (`s3://betterdfm-testdata/`) and pulled by CI.

To run integration tests locally:
```bash
aws s3 cp s3://betterdfm-testdata/rigidflex.tgz testdata/odb/
```

Files tracked here (committed):
- `README.md` — this file

Files NOT committed (add to .gitignore or download via CI):
- `*.tgz` — real ODB++ archives (potentially large or proprietary)

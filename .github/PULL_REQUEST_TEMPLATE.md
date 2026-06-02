<!--
Thanks for the PR! Fill in the sections below. Delete any that don't apply.
Keep the title short and conventional, e.g. feat(dfm): ..., fix(api): ..., docs: ...
-->

## What & why

<!-- What does this change do, and what problem does it solve? Link any issue. -->

Closes #

## Services touched

<!-- Check all that apply. Helps reviewers and the path-filtered deploy. -->

- [ ] web (`apps/web`)
- [ ] api (`apps/api`)
- [ ] worker (`workers/dfm-worker`)
- [ ] gerbonara sidecar (`sidecar/gerbonara`)
- [ ] dfm-engine (`engine/dfm-engine`)
- [ ] infra / CI / docs

## Changes

<!-- Bullet the notable changes. -->

-

## Reviewer notes

<!-- Anything non-obvious: trade-offs, heuristics, follow-ups, things you're unsure about. -->

## Checklist

- [ ] Go is formatted (`gofmt -l engine apps workers` is empty)
- [ ] Engine tests pass (`cd engine/dfm-engine && go test ./...`)
- [ ] api + worker build (`go build ./...`)
- [ ] Frontend typechecks and tests pass (`cd apps/web && pnpm test:run`)
- [ ] Sidecar tests pass if touched (`cd sidecar/gerbonara && pytest`)
- [ ] Docs updated if behavior/rules/profile changed (`CLAUDE.md`, `/technical` page)

<!--
DFM rule changes only:
- [ ] New rule registered in engine/dfm-engine/runner.go
- [ ] score.go ruleWeight + ruleMaxContribution updated; caps still sum to exactly 100
- [ ] New profile fields wired across engine types.go, api models.go, worker models.go, web api.ts, admin profile UI
-->

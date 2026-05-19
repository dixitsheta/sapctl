## Summary

<!-- 1-3 sentences. What changes and why. -->

## Linked issue

<!-- Closes #N. Or: "(no issue; trivial fix)" -->

## Test plan

- [ ] `go vet ./...` clean in every changed module
- [ ] `go test -race ./...` green
- [ ] If you touched the CLI surface: ran the affected command locally
- [ ] If you touched docs: previewed with `npm run dev`
- [ ] If you touched the audit chain or bundle layout: opened an ADR

## Risk

<!-- Anything that touches: auth, audit chain, bundle format, release
     pipeline, or locked flags / exit codes. Otherwise "low". -->

## Checklist

- [ ] Apache-2.0 contribution agreement understood
- [ ] No telemetry / analytics / phone-home introduced
- [ ] No SAP-copyrighted artifacts committed
- [ ] Locked exit codes (0/2/3/4/5/7) unchanged
- [ ] Conventional Commit subject

> Independent open-source project. Not affiliated with SAP SE.

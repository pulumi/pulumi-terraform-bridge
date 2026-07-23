# Network mirror — issue #3334

Design and plans for Terraform-style **network mirrors** in the dynamic TF provider path.

| File | Role |
|------|------|
| [design-concise.md](./design-concise.md) | Human-readable overview (same design, shortened) |
| [design.md](./design.md) | Full design — agents/implementers should read this |
| [maintainer-questions.md](./maintainer-questions.md) | Questions to post for maintainer feedback |
| [plan-phase-1-overrides-auth.md](./plan-phase-1-overrides-auth.md) | Impl plan: `MirrorSource` + `OVERRIDES` + `TF_TOKEN_*` |
| [plan-phase-2-provider-mirror-flag.md](./plan-phase-2-provider-mirror-flag.md) | Impl plan: `--provider-mirror` + `Value` (closes #3334) |

**Related:** [#3334](https://github.com/pulumi/pulumi-terraform-bridge/issues/3334), [pulumi-terraform-provider#106](https://github.com/pulumi/pulumi-terraform-provider/issues/106), [#3463](https://github.com/pulumi/pulumi-terraform-bridge/pull/3463) (superseded env-only approach).

**Order:** Phase 1 PR → Phase 2 PR. Phase 3 (credentials.json, hash verify, filesystem) deferred — no plan until demand.

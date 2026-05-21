# Doctor Issue Catalog

`agx doctor` reports issues using stable machine-readable `code` values.
This catalog is the canonical reference scripts should match against —
the human-readable `message` text and the `action` string may change
between releases, but `code` values follow the
[compatibility policy](../COMPATIBILITY.md).

A test (`internal/usecase/doctor_issue_catalog_test.go`) keeps this
file and the runtime emitters in sync: adding a new code that isn't
documented here makes the test fail, and removing a code that's still
emitted does too.

中文：[doctor-issues.zh.md](doctor-issues.zh.md).

## Severity

| Severity | Meaning |
| --- | --- |
| `error` | Something is broken; agx will not produce the right behavior until you fix it. |
| `warning` | Recoverable / pre-1.0 transitional. Action recommended but not required. |

## Codes

| Code | Severity | What it means | What to do |
| --- | --- | --- | --- |
| `unfinished_operation` | error | The previous mutating command (add / edit / use / activate / restore / …) crashed mid-flight; the journal still references it. | Run the suggested `agx restore <agent>`. agx auto-clears the journal entry once `restore` succeeds. |
| `missing_bound_profile` | error | An agent's native binding (`state.<agent>.SourceProfile`) names a profile that no longer exists in the AGX store. | Either `agx add <profile>` to re-create it or `agx detach <agent> <profile>` to clear the dangling binding. |
| `invalid_binding_status` | error | The persisted binding status for an agent is neither `applied` nor `bound`. | Re-run `agx use <profile>` for the agent (or `agx detach <agent> <profile>` to clear it). |
| `unconfigured_relay` | error | A profile is registered in the AGX store but not bound to any agent. | Either `agx run <agent> <profile>` (which also binds it) or `agx rm <profile>` if it's stale. |
| `orphan_derived_profile` | error | A derived profile (`codex.foo`, `opencode.foo`, …) exists in the store but no managed target references it. | Run the suggested `agx rm <derived>`. |
| `orphan_managed_target` | error | A managed target references a profile / derived profile that's missing. | Run the suggested `agx detach <agent> <name>`. |
| `model_id_drift` | error | An OpenCode-bound profile's `model` differs from the model recorded on the managed target. | Run `agx edit <profile> --model <id>` to re-sync the target. |
| `invalid_restore_mode` | error | A backup entry has a `restore_mode` value agx doesn't recognize (only `restore_file` / `remove_created_file` are valid). | The backup is unusable; remove it manually from state or recreate via `agx backup <agent>`. |
| `missing_backup_path` | error | A backup entry has no `backup_path` field, so restore would have nothing to read. | As above — drop the entry or replace with a fresh `agx backup <agent>`. |
| `missing_backup_file` | error | A backup entry's `backup_path` points to a file that no longer exists on disk. | Drop the entry; re-snapshot with `agx backup <agent>` if you still need a rollback point. |
| `runtime_binding_missing` | error | The agent's on-disk config has no AGX-managed block, but state thinks one exists. | Re-bind with `agx use <profile>` for that agent. |
| `runtime_binding_conflict` | error | The agent's on-disk config points at a different profile than state expects (someone edited the file out-of-band, or the agent CLI rewrote it). | Decide which side wins, then `agx use <profile>` to reconcile. |
| `runtime_binding_incomplete` | error | The AGX-managed block exists but is missing required fields. | Re-bind with `agx use <profile>` for that agent. |
| `runtime_config_unreadable` | error / warning | The native config file can't be parsed. **Warning** when v1 managed targets are still around (transitional state); otherwise **error**. | Fix the file (it's plain TOML / JSON / dotenv) or remove it so agx can rebuild from scratch. |

## Scripting

```bash
# bail on any error-severity issue
agx doctor -o json | jq -e '.issues | all(.severity != "error")' >/dev/null

# pull every action string for review
agx doctor -o json | jq -r '.issues[].action'

# filter to a specific code
agx doctor -o json | jq '.issues[] | select(.code == "orphan_managed_target")'
```

`agx doctor` exits non-zero iff at least one error-severity issue is
present. Warning-only reports exit `0`.

# Triage Labels

The skills speak in terms of five canonical triage roles. This file maps those roles to the mechanism this repo actually uses to record them.

As of [#621](https://github.com/markgoho/doula-cloud/issues/621), the mechanism is the **Status** field on the [Doula Cloud Project](https://github.com/users/markgoho/projects/5), not a label. See `docs/agents/issue-tracker.md` for how to read and write it.

| Role in mattpocock/skills | Status value on the Project | Meaning                                  |
| --------------------------- | ------------------------------ | ----------------------------------------- |
| `needs-triage`              | `Needs triage`                 | Maintainer needs to evaluate this issue  |
| `needs-info`                | `Needs info`                   | Waiting on reporter for more information |
| `ready-for-agent`           | `Ready for agent`              | Fully specified, ready for an AFK agent  |
| `ready-for-human`           | `Ready for human`              | Requires human implementation            |
| `wontfix`                   | *(no Status value)*            | Close the issue as "Not planned"; the `wontfix` label stays on the closed issue as the reason, it just doesn't route through Status |

Status also carries two values with no skill role. `Done` is set automatically by a Project workflow when the issue closes. `In progress` has no workflow behind it — set it by hand (`gh project item-edit`, see `docs/agents/issue-tracker.md`) at the same time you assign yourself the issue; assigning alone does not move Status.

When a skill mentions a role (e.g. "apply the AFK-ready triage label"), set the corresponding Status value from this table — do not add a label.

Edit the right-hand column to match whatever vocabulary you actually use.

# The repo goes private on Pro in December 2026, and CI runs on our own machines through runner-up

Status: accepted, 2026-11-08. Charted on [Wayfinder: private repo and self-hosted CI runners on our own machines (#2715)](https://github.com/markgoho/doula-cloud/issues/2715) and decided on [Going private and self-hosted runners: which jobs run where, and on which plan? (#2719)](https://github.com/markgoho/doula-cloud/issues/2719) and [Prototype: runner-up KVM places run inside WSL on the founder's Windows box, with the laptop as a second machine (#2742)](https://github.com/markgoho/doula-cloud/issues/2742), with evidence from [#2716](https://github.com/markgoho/doula-cloud/issues/2716), [#2717](https://github.com/markgoho/doula-cloud/issues/2717), [#2718](https://github.com/markgoho/doula-cloud/issues/2718) and [#2729](https://github.com/markgoho/doula-cloud/issues/2729). Written on [#2720](https://github.com/markgoho/doula-cloud/issues/2720).

`markgoho/doula-cloud` has been public for the whole build, and every CI job runs on a GitHub-hosted `ubuntu-26.04` runner, which costs nothing on a public repo. A private repo on a personal plan pays for minutes past an allowance, and loses rulesets and branch protection on Free. This records whether the repo goes private, on which plan, where each CI job runs once it is, and what holds the credentials.

## Visibility and when

- **The repo goes private in early December 2026**, before the January launch, so the runner path has a month of real use first.
- **The reason is to keep rivals from reading the roadmap**, the ADRs, the personas and the research.
- What has already been read stays read, as [ADR-0048](0048-nothing-holds-a-practice-here-but-the-next-family.md) says. Going private hides what is written from now on; it takes nothing back. ADR-0048's position does not rest on the repo being public, and it stands.
- **The restraints written because the repo is public stay** ([ADR-0045](0045-a-doula-is-heard-before-she-is-chosen.md), [ADR-0046](0046-the-product-counts-what-a-practice-did-and-never-who-she-is.md), [ADR-0047](0047-a-doula-sees-what-moves-her-and-her-own-words-do-the-moving.md)): the history up to the flip stays public in every clone and fork, and a private repo can be made public again. Nothing written after the flip is held to a looser rule.
- The interaction limit in `docs/agents/issue-tracker.md` is unnecessary once private and lapses on its own on 2027-04-21. The untrusted-outside-text rule in the same file stays, since Feedback, Dependabot and any later collaborator can still put outside text on an issue.

Decided on [#2719](https://github.com/markgoho/doula-cloud/issues/2719), item 1.

## Plan

- **GitHub Pro** (about $4/month, with $18/month of included compute). The account is on Free today.
- **The reason is the required checks.** The `Require PR on trunk` ruleset is what makes `gh pr merge --auto --squash` wait for the 11 required checks. On Free a private repo refuses a ruleset and branch protection outright (HTTP 403, measured on #2718), and an agent's PR would then merge before CI runs.
- If Pro refuses a ruleset on the private repo, the fallback is classic branch protection on `trunk` with "include administrators". GitHub's plans page lists protected branches for Pro private repos; its rulesets page names only Team and Enterprise. Which one Pro accepts is checked on the day (below).

Decided on [#2719](https://github.com/markgoho/doula-cloud/issues/2719), item 2. The cost evidence is [#2716's findings](https://github.com/markgoho/doula-cloud/issues/2716#issuecomment-6711040160): on hosted runners a private repo would cost about the same on Free or Pro, so the plan turns on features, and private hosted runs about 2x slower than public (#2718), which roughly doubles that estimate to about $250–430/month.

## The tool

- **runner-up** ([kunchenguid/runner-up](https://github.com/kunchenguid/runner-up), MIT): one coordinator holds GitHub's runner scale-set sessions and starts a one-job runner per job over SSH, in a fresh **KVM VM** (a "place") removed when the job ends. Docker places are not used: a VM is the stronger sandbox, and it keeps a Docker engine off the WSL box, where development runs on Podman.
- **The place image** is Ubuntu 26.04 with Docker, compose and buildx baked in, a pre-made `runner` user in the `docker` group, and a pinned Node. Without Node, vitest runs under Bun and a FormData identity test fails (#2729). CI runs Docker, not Podman (`CONTAINER_ENGINE: docker` in `ci.yml`), and that does not change.
- **Its risk is its age**: about ten days old when chosen, two contributors, on GitHub's preview scale-set API. The hosted fallback below is the hedge. If runner-up stops working, the next choice is plain `actions/runner --ephemeral` in a VM we script; ARC only with an always-on second machine.

Decided on [#2717's findings](https://github.com/markgoho/doula-cloud/issues/2717#issuecomment-6710973804), proved on [#2718](https://github.com/markgoho/doula-cloud/issues/2718#issuecomment-6716790827), [#2729](https://github.com/markgoho/doula-cloud/issues/2729#issuecomment-6717597865) and [#2742](https://github.com/markgoho/doula-cloud/issues/2742#issuecomment-6724649302).

## Which jobs run where

- **Every CI job runs on our own machines through `runs-on: ${{ vars.CI_LINUX || 'ubuntu-26.04' }}`.** An unset variable means GitHub-hosted, so the switch is one repo variable.
- **Every job that signs in to Google Cloud through Workload Identity Federation stays GitHub-hosted:** in `ci.yml`, `migrate`, `deploy-api`, `deploy-app`, `deploy-ops` and `deploy-ops-site`; `terraform-plan.yml`'s `plan` and `plan-stripe-sandbox`; `firebase-hosting-merge.yml`; and `hosting-edge-check.yml`. Three reasons: a trunk merge must deploy even when both home machines are down; a production token is never minted on a machine that runs untrusted dependency code; and these jobs are short enough to fit roughly within Pro's included compute.
- **The sub-minute jobs merge** (format, actionlint, docs, scripts, secrets, site and the like), because each bills a full minute on its own (#2716) and there is no memory left for a small-job pool (#2729). The spec decides the exact grouping, since `changes` gates downstream jobs and `trunk-red.yml` runs on `workflow_run`.

Decided on [#2719](https://github.com/markgoho/doula-cloud/issues/2719), item 3. The Firebase Hosting PR-preview workflows were on this list until [#2775](https://github.com/markgoho/doula-cloud/issues/2775) removed them as unused, 2026-11-08.

## Machines, coordinator and credentials

- **The coordinator runs on the founder's WSL box** (distro `doula`, 16 threads, 47 GB, `/dev/kvm`), as a systemd user service with linger. It is on around the clock apart from Windows Update reboots, development happens there, and it already holds the founder's admin `gh` sign-in. runner-up takes a token from `gh auth token` per session and stores none, so **no admin token sits on the laptop**.
- **The basement laptop does the heavy work**: a Dell Precision 7530 (6 cores / 12 threads, 15.7 GB) running Omarchy, dedicated, never sleeping or traveling, with two places of 5 cores / 5.5 GB each. Two places ran `app` and `api` together, green, in 10.3 min (#2729). `app` peaks at 86% of 5.5 GB, so that is the floor.
- **The WSL box has one place, capped at 8 cores / 12 GB, as overflow.** It takes jobs when both laptop places are busy or the laptop is down. Nested KVM inside WSL2 runs about 1.9x slower than the laptop (#2742), which is why it is overflow and not the main machine. Tuning nested KVM was rejected.
- **No cloud credential is on either machine.** The WIF jobs above are the only ones that reach Google Cloud.

Decided on [#2719](https://github.com/markgoho/doula-cloud/issues/2719), item 4, and [#2742](https://github.com/markgoho/doula-cloud/issues/2742), item 1.

## When a machine is down

- **Laptop unreachable:** its places rest and jobs go to the WSL place, with no one touching it (shown on #2742).
- **WSL box off:** PR CI waits until it is back. For an exception the founder flips by hand, with `gh variable set CI_LINUX --body ubuntu-26.04`, and undoes it with `gh variable delete CI_LINUX`. No auto-flip, and no extra token for one.
- **After a Windows Update reboot**, CI is down until the founder logs in to Windows, because the WSL keepalive runs at login. Accepted, since CI is only needed while developing. No Windows auto-login.
- **After a power cut**, the laptop waits at its LUKS passphrase prompt (the Omarchy password screen) until the founder types it. Accepted; no TPM enrollment and no remote unlock.
- **A suspend mid-job fails that job** with no retry ("runner lost communication", #2718). The laptop is set never to suspend.
- **Revisit** this section if Claude cloud sessions or other contributors start opening PRs while the WSL box is off.

Decided on [#2719](https://github.com/markgoho/doula-cloud/issues/2719), items 5 and 6, and [#2742](https://github.com/markgoho/doula-cloud/issues/2742), item 2.

## What the build spec carries

The flip and the runner build are specified from this ADR through `/to-spec` and `/to-tickets`. The spec carries:

- The `runs-on` variable on every non-WIF job, and the merged sub-minute job.
- The place image recipe: #2718's image plus the `runner` user and a pinned Node.
- The coordinator's systemd user service on WSL, with linger, and its config.
- On the laptop: sshd listening on the LAN, key-only; the ufw rule allowing only the WSL box's address on port 22; logind's `HandleLidSwitch=ignore`, with the coordinator's side wrapped so the machine never sleeps; and network isolation so a job cannot reach the home LAN (a place reaches the LAN and the host's loopback through QEMU user networking).
- DHCP reservations in the Deco app for the laptop `dellarchy` (192.168.68.78) and the WSL box's Ethernet adapter (192.168.68.89, MAC `D8:BB:C1:1A:AB:1D`). Without them, sshd stops binding or the ufw rule stops matching when an address changes.

### The flip-day checklist

- Upgrade to Pro, make the repo private, then confirm the `Require PR on trunk` ruleset is still accepted and enforced. If not, set classic branch protection on `trunk` with "include administrators" and the same 11 required checks.
- Confirm a `repository_dispatch` still lands with `GITHUB_DISPATCH_TOKEN` (`docs/environment.md`), a fine-grained token scoped to this repo with Contents: write.
- Rewrite the text that says the repo is public: `docs/agents/issue-tracker.md`'s "The repo is public" and its interaction-limit paragraph.
- Decide each public link into the repo that a reader outside it can no longer open, such as the `#338` link on `app/src/routes/style-guide/brand-lockup/+page.svelte`.

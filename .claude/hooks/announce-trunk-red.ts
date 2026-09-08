#!/usr/bin/env bun
// Announce a red trunk at the top of every session (#1021).
//
// CI on trunk runs `migrate` against the real doula-cloud-pg and then
// `deploy-api`/`deploy-app`. None of those runs on a pull request, so a
// green PR says nothing about them and a session that only ever watches
// PR checks will never learn trunk is broken. #967 broke `migrate` and
// trunk stayed red across seven merges for two and a half hours, because
// every PR in that window was green.
//
// `.github/workflows/trunk-red.yml` is the durable half of the alarm: it
// files an assigned issue. This is the half that reaches an agent, which
// does not read GitHub notifications -- it puts the failure in front of a
// session before it branches from a trunk that does not build or deploy.
//
// Deliberately advisory, never blocking: red trunk is often exactly what
// the session has been opened to fix, and a hook that refused to let work
// start would be the wrong tool. It also fails open on any error -- no
// network, no gh, rate-limited -- because a hook that breaks a session
// over its own diagnostics is worse than one that stays quiet.

const REPO = "markgoho/doula-cloud";

type Run = {
  conclusion: string | null;
  status: string;
  head_sha: string;
  html_url: string;
  created_at: string;
};

async function sh(cmd: string[]): Promise<string | null> {
  try {
    const p = Bun.spawn(cmd, { stdout: "pipe", stderr: "pipe" });
    const out = await new Response(p.stdout).text();
    if ((await p.exited) !== 0) return null;
    return out;
  } catch {
    return null;
  }
}

const raw = await sh([
  "gh",
  "api",
  `repos/${REPO}/actions/runs?branch=trunk&per_page=10`,
  "--jq",
  ".workflow_runs[] | select(.name==\"CI\") | {conclusion, status, head_sha, html_url, created_at}",
]);

if (!raw) process.exit(0);

const runs: Run[] = raw
  .trim()
  .split("\n")
  .filter(Boolean)
  .flatMap((line) => {
    try {
      return [JSON.parse(line) as Run];
    } catch {
      return [];
    }
  });

const latest = runs.find((r) => r.status === "completed");
if (!latest || latest.conclusion !== "failure") process.exit(0);

// How long it has been red: count back to the newest completed run that
// succeeded. A single red run is a fresh break; a streak means merges have
// been landing on top of it, which is the state worth shouting about.
let streak = 0;
for (const r of runs) {
  if (r.status !== "completed") continue;
  if (r.conclusion === "failure") streak++;
  else break;
}

const failing = await sh([
  "gh",
  "api",
  `repos/${REPO}/actions/runs?branch=trunk&per_page=1&status=failure`,
  "--jq",
  ".workflow_runs[0].id",
]);

let jobs = "";
if (failing) {
  const j = await sh([
    "gh",
    "api",
    `repos/${REPO}/actions/runs/${failing.trim()}/jobs`,
    "--jq",
    '[.jobs[] | select(.conclusion=="failure") | .name] | join(", ")',
  ]);
  if (j?.trim()) jobs = j.trim();
}

const lines = [
  "TRUNK IS RED.",
  `  Latest trunk CI failed on ${latest.head_sha.slice(0, 8)}${jobs ? ` — failing job(s): ${jobs}` : ""}.`,
  streak > 1
    ? `  It has failed ${streak} runs in a row, so merges are landing on a broken trunk.`
    : "  This is the first failing run.",
  `  ${latest.html_url}`,
  "",
  "  `migrate`, `deploy-api` and `deploy-app` run ONLY on trunk, so a green PR",
  "  proves nothing about them. While trunk is red nothing is deploying, and a",
  "  branch cut from it inherits the breakage.",
  "",
  "  Fix or triage this before landing unrelated work. See the open `trunk-red`",
  "  issue, filed automatically by .github/workflows/trunk-red.yml.",
];

console.error(lines.join("\n"));
process.exit(0);

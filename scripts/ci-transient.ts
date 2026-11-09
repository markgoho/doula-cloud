// Whether a failed workflow run failed on infrastructure and not on code
// (#2819). `scripts/ci-heal.ts` reruns a run that this calls transient, and
// `scripts/trunk-red.ts` opens no alarm for one, so both read one verdict.
//
// A run is transient only when every failed job failed in a step that
// sets up or downloads, and the log of that step holds a signature of a
// network or host failure. A test, a lint or a build step is not on the
// list of steps, so it is never transient whatever its log says.
import { spawnSync } from 'node:child_process';

export type Step = { name: string; conclusion: string | null };
export type Job = {
  id: number;
  name: string;
  conclusion: string | null;
  steps?: Step[];
};
export type Verdict = { transient: boolean; reason: string };

/** The steps a runner or an action runs before the job's own work. */
const RUNNER_STEP = /^(Set up job|Initialize containers)$/;

/**
 * The steps whose failure can be a download. The allowlist is the guard:
 * a step that no entry names is real work, and a failure there is a
 * person's to read.
 */
const SETUP_STEP =
  /^Run (\.\/\.github\/actions\/|actions\/|oven-sh\/|hashicorp\/|google-github-actions\/|docker\/)|\b(install|download|pull|init|set ?up|cache|checkout|restore|authenticate|validate)\b|^Terraform|^Plan\b/i;

/** What a network or a host failure writes, in the logs of the tools this repository runs. */
const SIGNATURES: RegExp[] = [
  /Unexpected HTTP response: 5\d\d/,
  /\b50[0234] (?:Bad Gateway|Service (?:Temporarily )?Unavailable|Gateway Time-?out|Internal Server Error)/i,
  /\bHTTP(?:\/\d(?:\.\d)?)? 5\d\d\b/,
  /\b(?:status(?: code)?|response|error)[: =]+5\d\d\b/i,
  /\b(?:ETIMEDOUT|ECONNRESET|ECONNREFUSED|EAI_AGAIN|ESOCKETTIMEDOUT)\b/,
  /RemoteDisconnected/,
  /TLS handshake timeout|i\/o timeout|connection reset by peer|Client\.Timeout exceeded/,
];

const ANSI = /\x1b\[[0-9;]*m/g;
/** How many lines before an `##[error]` line can hold its cause. */
const WINDOW = 30;

/**
 * The first signature in the lines that lead up to an error annotation,
 * the only part of a log that belongs to a failure. A signature in the
 * output of a step that passed is not evidence.
 */
export function signatureBeforeError(log: string): string | null {
  const lines = log.replace(ANSI, '').split('\n');
  for (let i = 0; i < lines.length; i++) {
    if (!lines[i]?.includes('##[error]')) continue;
    for (const line of lines.slice(Math.max(0, i - WINDOW), i + 1)) {
      if (SIGNATURES.some((signature) => signature.test(line))) {
        return line.replace(/^\S+Z /, '').trim();
      }
    }
  }
  return null;
}

/** The verdict for a failed run's jobs, each with the log it wrote. */
export function classify(jobs: (Job & { log: string })[]): Verdict {
  const failed = jobs.filter((job) => job.conclusion === 'failure');
  if (failed.length === 0) {
    return { transient: false, reason: 'no job has the conclusion failure' };
  }
  const found: string[] = [];
  for (const job of failed) {
    const steps = (job.steps ?? []).filter(
      (step) => step.conclusion === 'failure'
    );
    if (steps.length === 0) {
      return {
        transient: false,
        reason: `${job.name} failed with no failed step`,
      };
    }
    const real = steps.find(
      (step) => !RUNNER_STEP.test(step.name) && !SETUP_STEP.test(step.name)
    );
    if (real) {
      return {
        transient: false,
        reason: `${job.name} failed at step "${real.name}", which is not a setup or download step`,
      };
    }
    if (steps.every((step) => RUNNER_STEP.test(step.name))) {
      found.push(`${job.name}: runner step "${steps[0]?.name}" failed`);
      continue;
    }
    const signature = signatureBeforeError(job.log);
    if (!signature) {
      return {
        transient: false,
        reason: `${job.name} failed at "${steps[0]?.name}" with no transient signature in the log`,
      };
    }
    found.push(`${job.name}: "${steps[0]?.name}" - ${signature}`);
  }
  return { transient: true, reason: found.join('; ') };
}

function gh(args: string[]): string {
  const result = spawnSync(process.env.GH_BIN || 'gh', args, {
    encoding: 'utf8',
    maxBuffer: 256 * 1024 * 1024,
  });
  if (result.status !== 0) {
    throw new Error(
      `gh ${args.join(' ')} failed (exit ${result.status}): ${result.stderr || result.error?.message || ''}`
    );
  }
  return result.stdout;
}

/**
 * Reads the failed attempt's jobs and their logs, and classifies them. It
 * throws when a read fails; each caller decides what that means for it.
 */
export function classifyRun(
  repo: string,
  run: { id: number; run_attempt?: number }
): Verdict {
  const jobs =
    (
      JSON.parse(
        gh([
          'api',
          `repos/${repo}/actions/runs/${run.id}/attempts/${run.run_attempt ?? 1}/jobs?per_page=100`,
        ])
      ) as { jobs?: Job[] }
    ).jobs ?? [];
  const withLogs = jobs.map((job) => ({
    ...job,
    log:
      job.conclusion === 'failure'
        ? gh([
            'api',
            '--allow-escape-sequences',
            `repos/${repo}/actions/jobs/${job.id}/logs`,
          ])
        : '',
  }));
  return classify(withLogs);
}

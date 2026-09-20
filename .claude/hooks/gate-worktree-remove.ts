#!/usr/bin/env bun
// PreToolUse gate on Bash: refuses a `git worktree remove` that would
// destroy work someone else is still doing (#1212).
//
// On 2026-09-10 an orchestrating session short of port offsets checked
// every signal git offers -- bare `agent-<id>` branch, no commits past
// trunk, clean `git status` -- and ran `git worktree remove --force` on a
// worktree that was a live agent twenty minutes into its task. The pruner
// cannot guard a removal typed by hand, so this gate does. It refuses:
//
//   - a worktree whose recorded owner (worktree-owner.ts) is a Claude
//     Code process that is still running and is not this session. `--force`
//     does not get past this: the incident used `--force`.
//   - `--force` on a worktree holding uncommitted changes, whoever owns
//     it. Without `--force` git refuses a dirty tree itself, so that case
//     is left to git.
//
// The deliberate override is to prefix the command with
// `ALLOW_LIVE_WORKTREE_REMOVE=1` -- visible in the transcript, never an
// accident. An owner that is gone or unknown (a worktree provisioned
// before #1212) is not refused on ownership; the dirty rule still holds.
//
// Fails closed, like gate-worktree-edit.ts: a gate that crashed has not
// decided the removal is safe. docs/agents/worktree-flow.md has the rule.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { readStdin } from './tracked-path.ts';
import { describeOwner, findClaudePid, ownerStatus } from './worktree-owner.ts';

const OVERRIDE = 'ALLOW_LIVE_WORKTREE_REMOVE=1';

interface RemoveRequest {
  target: string;
  force: boolean;
}

function unquote(token: string): string {
  return token.replace(/^(['"])(.*)\1$/, '$2');
}

// Every `git [global options] worktree remove [options] <path>` in the
// command, one per shell segment.
function findRemovals(command: string): RemoveRequest[] {
  const found: RemoveRequest[] = [];
  for (const segment of command.split(/&&|\|\||[;|\n]/)) {
    const tokens = segment.trim().split(/\s+/).map(unquote);
    const gitAt = tokens.indexOf('git');
    if (gitAt === -1) continue;
    let i = gitAt + 1;
    while (i < tokens.length && tokens[i]?.startsWith('-')) {
      i += tokens[i] === '-C' || tokens[i] === '-c' ? 2 : 1;
    }
    if (tokens[i] !== 'worktree' || tokens[i + 1] !== 'remove') continue;
    let force = false;
    let target: string | undefined;
    for (const token of tokens.slice(i + 2)) {
      if (token === '-f' || token === '--force') force = true;
      else if (!token.startsWith('-') && target === undefined) target = token;
    }
    if (target) found.push({ target, force });
  }
  return found;
}

function isDirty(worktree: string): boolean {
  return (
    execFileSync('git', ['-C', worktree, 'status', '--porcelain'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    }).trim().length > 0
  );
}

function refusal(request: RemoveRequest, cwd: string): string | null {
  const worktree = path.resolve(cwd, request.target);
  // Nothing there: git will say so, and there is nothing to lose.
  if (!fs.existsSync(worktree)) return null;

  const owner = ownerStatus(worktree);
  if (owner.state === 'alive' && owner.owner.pid !== findClaudePid()) {
    return (
      `${worktree} belongs to a Claude Code session that is still running: ${describeOwner(owner)}.\n` +
      'An agent may be working in it right now without having committed yet -- ' +
      'a clean tree and a branch with no commits look exactly like that.\n' +
      'Leave it for its session to remove. If you have confirmed that process is not working ' +
      `here (ps -p ${owner.owner.pid}), prefix the command with ${OVERRIDE}.`
    );
  }
  if (request.force && isDirty(worktree)) {
    return (
      `${worktree} holds uncommitted changes, and --force would destroy them.\n` +
      `Read them first (git -C ${worktree} status). To discard them deliberately, prefix the command with ${OVERRIDE}.`
    );
  }
  return null;
}

async function main(): Promise<void> {
  const raw = await readStdin();
  let payload: Record<string, unknown> = {};
  if (raw.trim()) {
    try {
      payload = JSON.parse(raw);
    } catch {
      process.exit(0);
    }
  }
  if (payload['tool_name'] !== 'Bash') process.exit(0);
  const toolInput = payload['tool_input'] as
    Record<string, unknown> | undefined;
  const command = toolInput?.['command'];
  if (typeof command !== 'string' || command.includes(OVERRIDE))
    process.exit(0);

  const cwd =
    typeof payload['cwd'] === 'string' ? payload['cwd'] : process.cwd();
  for (const request of findRemovals(command)) {
    const reason = refusal(request, cwd);
    if (reason) {
      process.stdout.write(JSON.stringify({ decision: 'block', reason }));
      process.exit(2);
    }
  }
}

main().catch((error) => {
  process.stdout.write(
    JSON.stringify({
      decision: 'block',
      reason: `gate-worktree-remove could not check this removal (${error instanceof Error ? error.message : String(error)}), so it is refused. Prefix the command with ${OVERRIDE} if you have checked the worktree yourself.`,
    })
  );
  process.exit(2);
});

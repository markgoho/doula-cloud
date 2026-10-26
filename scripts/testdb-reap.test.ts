/**
 * `.claude/hooks/testdb-reap.ts` -- the SessionStart hook that reaps the
 * containers a killed `go test` process leaves behind (#889, #2096). See
 * that file's header and docs/testing/api.md's "Reaping orphaned
 * testcontainers" section for the full story.
 *
 * The reap decision (pickReapCandidates) is pure, so it's imported and
 * unit tested directly with a fake liveness test. main() -- the engine
 * calls, `--dry-run` and the fail-open catch -- runs as a subprocess
 * against a fake engine binary that prints canned `ps` output and logs
 * every call, so no real container is ever touched here.
 */
import { afterAll, describe, expect, test } from 'bun:test';
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {
  OWNER_LABEL,
  OWNER_PID_LABEL,
  ownerPid,
  pickReapCandidates,
} from '../.claude/hooks/testdb-reap.ts';
import type { ReapCandidate } from '../.claude/hooks/container-engine.ts';

const REPO_ROOT = path.resolve(import.meta.dir, '..');
const HOOK = path.join(REPO_ROOT, '.claude', 'hooks', 'testdb-reap.ts');

function container(id: string, labels: Record<string, string>): ReapCandidate {
  return { id, name: `name_${id}`, labels, createdAtMs: 0 };
}

function owned(id: string, pid: string): ReapCandidate {
  return container(id, { [OWNER_LABEL]: 'true', [OWNER_PID_LABEL]: pid });
}

const DEAD = 4242;
const LIVE = 1717;
const isAlive = (pid: number) => pid === LIVE;

describe('pickReapCandidates', () => {
  test('removes a labeled container whose pid is gone', () => {
    const orphan = owned('a', String(DEAD));
    expect(pickReapCandidates([orphan], isAlive)).toEqual([orphan]);
  });

  test('keeps a labeled container whose pid is alive', () => {
    expect(pickReapCandidates([owned('a', String(LIVE))], isAlive)).toEqual([]);
  });

  test('keeps a container with no owner marker, even a testcontainers one', () => {
    const unlabeled = container('a', {
      'org.testcontainers': 'true',
      [OWNER_PID_LABEL]: String(DEAD),
    });
    expect(pickReapCandidates([unlabeled], isAlive)).toEqual([]);
  });

  test('keeps an e2e compose stack container', () => {
    const stack = container('a', {
      'com.docker.compose.project': 'doula-cloud-e2e-2',
      [OWNER_LABEL]: 'true',
      [OWNER_PID_LABEL]: String(DEAD),
    });
    expect(pickReapCandidates([stack], isAlive)).toEqual([]);
  });

  test('keeps a container whose marker is not exactly "true"', () => {
    const odd = container('a', {
      [OWNER_LABEL]: 'yes',
      [OWNER_PID_LABEL]: String(DEAD),
    });
    expect(pickReapCandidates([odd], isAlive)).toEqual([]);
  });

  test('keeps a marked container whose pid label is missing or not a positive integer', () => {
    const cases = [
      container('a', { [OWNER_LABEL]: 'true' }),
      owned('b', ''),
      owned('c', '0'),
      owned('d', '-5'),
      owned('e', '12x'),
      owned('f', '012'),
      owned('g', '99999999999999999999'),
    ];
    expect(pickReapCandidates(cases, () => false)).toEqual([]);
  });

  test('only removes the orphans out of a mixed list', () => {
    const orphan = owned('a', String(DEAD));
    const live = owned('b', String(LIVE));
    const unlabeled = container('c', {});
    expect(pickReapCandidates([live, orphan, unlabeled], isAlive)).toEqual([
      orphan,
    ]);
  });

  test('defaults to the real liveness test: this process is alive', () => {
    expect(pickReapCandidates([owned('a', String(process.pid))])).toEqual([]);
  });
});

describe('ownerPid', () => {
  test('reads a plain decimal pid', () => {
    expect(ownerPid(owned('a', '123'))).toBe(123);
  });
});

// A pid that has certainly exited: a child we spawned and waited for.
function exitedPid(): number {
  const child = spawnSync('true');
  return child.pid;
}

const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'testdb-reap-2096-'));
afterAll(() => fs.rmSync(scratch, { recursive: true, force: true }));

function fakeEngine(psJson: string): { binary: string; log: string } {
  const dir = fs.mkdtempSync(path.join(scratch, 'engine-'));
  const psFile = path.join(dir, 'ps.json');
  const log = path.join(dir, 'calls.log');
  fs.writeFileSync(psFile, psJson);
  const binary = path.join(dir, 'engine');
  fs.writeFileSync(
    binary,
    `#!/bin/sh\necho "$@" >> '${log}'\ncase "$1" in ps) cat '${psFile}';; esac\n`,
    { mode: 0o755 }
  );
  return { binary, log };
}

function invoke(
  env: Record<string, string | undefined>,
  args: string[] = []
): Promise<{ exitCode: number; stdout: string }> {
  return new Promise((resolve, reject) => {
    const child = spawn('bun', [HOOK, ...args], {
      stdio: ['ignore', 'pipe', 'inherit'],
      env,
    });
    let stdout = '';
    child.stdout.on('data', (chunk) => {
      stdout += chunk;
    });
    child.on('error', reject);
    child.on('close', (code) => resolve({ exitCode: code ?? 1, stdout }));
  });
}

function envFor(binary: string): Record<string, string | undefined> {
  const env: Record<string, string | undefined> = {
    ...process.env,
    CONTAINER_ENGINE: binary,
  };
  delete env.DOCKER_HOST;
  return env;
}

function psJson(): string {
  const entry = (id: string, labels: Record<string, string>) => ({
    Id: id,
    Names: [`name_${id}`],
    Labels: labels,
    Created: 0,
  });
  return JSON.stringify([
    entry('orphan000000aa', {
      [OWNER_LABEL]: 'true',
      [OWNER_PID_LABEL]: String(exitedPid()),
    }),
    entry('live00000000bb', {
      [OWNER_LABEL]: 'true',
      [OWNER_PID_LABEL]: String(process.pid),
    }),
    entry('stack0000000cc', {
      'com.docker.compose.project': 'doula-cloud-e2e-2',
    }),
  ]);
}

describe('testdb-reap hook (subprocess, fake engine)', () => {
  test('--dry-run lists only the orphan and removes nothing', async () => {
    const engine = fakeEngine(psJson());
    const { exitCode, stdout } = await invoke(envFor(engine.binary), [
      '--dry-run',
    ]);
    expect(exitCode).toBe(0);
    expect(stdout).toContain('would remove 1 orphaned test container(s)');
    expect(stdout).toContain('orphan000000 name_orphan000000aa');
    expect(stdout).not.toContain('live');
    expect(stdout).not.toContain('stack');
    const calls = fs.readFileSync(engine.log, 'utf8').trim().split('\n');
    expect(calls).toEqual([
      `ps -a --filter label=${OWNER_LABEL}=true --format json`,
    ]);
  });

  test('a real run removes exactly the orphan', async () => {
    const engine = fakeEngine(psJson());
    const { exitCode, stdout } = await invoke(envFor(engine.binary));
    expect(exitCode).toBe(0);
    expect(stdout).toContain('removed 1 orphaned test container(s)');
    const calls = fs.readFileSync(engine.log, 'utf8').trim().split('\n');
    expect(calls[1]).toBe('rm -f -t 2 orphan000000aa');
    expect(calls).toHaveLength(2);
  });

  test('stays quiet and removes nothing when there is no orphan', async () => {
    const engine = fakeEngine('[]');
    const { exitCode, stdout } = await invoke(envFor(engine.binary));
    expect(exitCode).toBe(0);
    expect(stdout).toBe('');
    expect(fs.readFileSync(engine.log, 'utf8').trim().split('\n')).toHaveLength(
      1
    );
  });

  test('fails open when the container engine is unreachable', async () => {
    const { exitCode, stdout } = await invoke({
      ...process.env,
      DOCKER_HOST: 'unix:///nonexistent/testdb-reap-test.sock',
      CONTAINER_ENGINE: '/nonexistent-binary-testdb-reap-test',
    });
    expect(exitCode).toBe(0);
    expect(stdout).toBe('');
  });
});

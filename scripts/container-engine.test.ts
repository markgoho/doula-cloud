/**
 * `.claude/hooks/container-engine.ts` -- how the two SessionStart reapers
 * (testdb-reap.ts, e2e-stack-reap.ts) reach the local container engine.
 *
 * The regression this pins down: testdb-reap.ts used to return early when
 * DOCKER_HOST was unset, on the reading that an unset variable meant "no
 * container engine here". docs/testing/api.md has DOCKER_HOST exported by
 * hand into the shell that runs the tests, never from a login profile, so
 * a hook -- which inherits the login environment -- never saw it and the
 * reaper had never once run. Confirmed live: 38-hour-old
 * `org.testcontainers=true` containers on a machine that had started
 * dozens of sessions since. `podman ps` with no `--url` reaches the same
 * engine through its default machine connection, so the fix is to drop
 * the flag, not the run.
 */
import { afterEach, describe, expect, test } from 'bun:test';
import {
  engineBinary,
  engineInvocation,
  execEngine,
  parseContainers,
} from '../.claude/hooks/container-engine.ts';

const originalEngine = process.env.CONTAINER_ENGINE;
const originalHost = process.env.DOCKER_HOST;

function restore(
  name: 'CONTAINER_ENGINE' | 'DOCKER_HOST',
  value: string | undefined
): void {
  if (value === undefined) delete process.env[name];
  else process.env[name] = value;
}

afterEach(() => {
  restore('CONTAINER_ENGINE', originalEngine);
  restore('DOCKER_HOST', originalHost);
});

describe('engineBinary', () => {
  test('defaults to podman', () => {
    delete process.env.CONTAINER_ENGINE;
    expect(engineBinary()).toBe('podman');
  });

  test('honors CONTAINER_ENGINE, the same variable app/e2e/stack.ts reads', () => {
    process.env.CONTAINER_ENGINE = 'docker';
    expect(engineBinary()).toBe('docker');
  });
});

describe('engineInvocation', () => {
  test('points the engine at DOCKER_HOST when one is set', () => {
    process.env.DOCKER_HOST = 'unix:///run/user/501/podman/podman.sock';
    expect(engineInvocation(['ps', '-a'])).toEqual({
      binary: engineBinary(),
      argv: ['--url', 'unix:///run/user/501/podman/podman.sock', 'ps', '-a'],
    });
  });

  test('still invokes the engine with no DOCKER_HOST, letting it use its default connection', () => {
    delete process.env.DOCKER_HOST;
    expect(engineInvocation(['ps', '-a']).argv).toEqual(['ps', '-a']);
  });

  test('treats an empty DOCKER_HOST as unset', () => {
    process.env.DOCKER_HOST = '';
    expect(engineInvocation(['ps', '-a']).argv).toEqual(['ps', '-a']);
  });
});

describe('execEngine', () => {
  function recorder(failWhen: (argv: string[]) => boolean) {
    const calls: string[][] = [];
    const run = (_binary: string, argv: string[]) => {
      calls.push(argv);
      if (failWhen(argv)) throw new Error('engine refused');
      return 'ok';
    };
    return { calls, run };
  }

  test('uses DOCKER_HOST first and stops there when it works', () => {
    process.env.DOCKER_HOST = 'unix:///podman.sock';
    const { calls, run } = recorder(() => false);
    expect(execEngine(['ps'], run)).toBe('ok');
    expect(calls).toEqual([['--url', 'unix:///podman.sock', 'ps']]);
  });

  test('falls back to the default connection when DOCKER_HOST names another engine (#2096)', () => {
    process.env.DOCKER_HOST = 'unix:///Users/me/.colima/default/docker.sock';
    const { calls, run } = recorder((argv) => argv[0] === '--url');
    expect(execEngine(['ps'], run)).toBe('ok');
    expect(calls).toEqual([
      ['--url', 'unix:///Users/me/.colima/default/docker.sock', 'ps'],
      ['ps'],
    ]);
  });

  test('tries once, with no repeat, when DOCKER_HOST is unset', () => {
    delete process.env.DOCKER_HOST;
    const { calls, run } = recorder(() => true);
    expect(() => execEngine(['ps'], run)).toThrow('engine refused');
    expect(calls).toEqual([['ps']]);
  });

  test('throws when both forms fail', () => {
    process.env.DOCKER_HOST = 'unix:///nowhere.sock';
    const { calls, run } = recorder(() => true);
    expect(() => execEngine(['ps'], run)).toThrow('engine refused');
    expect(calls).toHaveLength(2);
  });
});

describe('parseContainers', () => {
  test('maps an engine ps --format json array into ReapCandidates', () => {
    const json = JSON.stringify([
      {
        Id: 'deadbeef',
        Names: ['clever_name'],
        Labels: { 'org.testcontainers': 'true' },
        Created: 1000,
      },
    ]);
    expect(parseContainers(json)).toEqual([
      {
        id: 'deadbeef',
        name: 'clever_name',
        labels: { 'org.testcontainers': 'true' },
        createdAtMs: 1_000_000,
      },
    ]);
  });

  test('falls back to the id when Names is absent, and to an empty label set when Labels is absent', () => {
    const json = JSON.stringify([{ Id: 'deadbeef', Created: 0 }]);
    expect(parseContainers(json)).toEqual([
      { id: 'deadbeef', name: 'deadbeef', labels: {}, createdAtMs: 0 },
    ]);
  });
});

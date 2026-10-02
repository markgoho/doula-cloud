#!/usr/bin/env bash
# Installs the OS libraries Playwright's Chromium links against, with a
# bounded, retryable apt (#1655). One script for the three CI jobs that need
# it (`site` and `app` in ci.yml, gcp-dashboard-ci.yml), so the fix cannot
# drift between them. Run it from the directory that holds the Playwright
# dependency (`bunx playwright` resolves the version there).
#
# It runs on every job run, on a Playwright cache hit and on a miss alike
# (#1661). The browser download that follows it on a miss passes no
# `--with-deps`, because that flag starts a root `apt-get` through `sudo`
# with none of the bounds below. The OS libraries are installed here, once.
#
# Why a bare `timeout ... bunx playwright install-deps` was not enough:
# `timeout` runs as the unprivileged `runner` user, and Playwright starts
# `apt-get` through `sudo`, so `apt-get` runs as root. After the timeout
# fires it stops `bunx` but cannot signal the root `apt-get`, which keeps
# running and keeps the dpkg lock. Every later attempt then died in about a
# second on `E: Could not get lock /var/lib/dpkg/lock-frontend` (run
# 37025325343, a mirror serving 32 MB at about 80 kB/s).
#
# The fix leaves the orphan alone and makes the retry wait for it. The
# orphan keeps downloading and installs; the retry then finds the packages
# already installed. Choosing "wait" over "kill it as root" avoids a
# half-configured dpkg, which would need `dpkg --configure -a` to repair.
#   1. apt's own bounds go in an apt.conf.d file (not env vars: sudo strips
#      the environment, a file reaches the root apt-get):
#        Acquire::http(s)::Timeout  idle time on one connection before apt
#                                   drops it; a stalled connection ends in
#                                   apt, not in an outside kill.
#        Acquire::Retries           apt re-tries a failed download itself.
#        DPkg::Lock::Timeout        a second apt-get waits for the dpkg lock
#                                   instead of failing at once.
#   2. Each attempt still has an outer `timeout`, because the idle timeout
#      does not end a connection that is slow but alive.
#   3. After a failed attempt the script waits (bounded) until no apt-get or
#      dpkg process is left, then retries with an uncontended lock.
#
# The whole step is capped at TOTAL_BUDGET (540s): worst case 180s + wait +
# 180s would otherwise be unbounded. The three calling jobs carry a
# 20-minute timeout, which holds this budget plus the browser download's
# worst case (3 x 120s + 2 x 30s); the arithmetic is on ci.yml's `app` job.
#
# Every knob below can be overridden by an environment variable; the spec in
# scripts/install-playwright-deps.test.ts uses that to run the loop against
# fake commands.
set -u

install_cmd="${INSTALL_CMD:-bunx playwright install-deps chromium}"
attempts="${ATTEMPTS:-3}"
attempt_timeout="${ATTEMPT_TIMEOUT:-180}"
retry_sleep="${RETRY_SLEEP:-10}"
settle_timeout="${SETTLE_TIMEOUT:-240}"
# The whole script gives up after this many seconds, so the worst case fits
# the 20-minute job timeout together with the browser download.
total_budget="${TOTAL_BUDGET:-540}"
poll_interval="${POLL_INTERVAL:-5}"
apt_conf="${APT_CONF:-/etc/apt/apt.conf.d/99-ci-bounds}"
timeout_cmd="${TIMEOUT_CMD:-timeout}"
as_root="${AS_ROOT-sudo}"
# The files `apt_busy` probes with `fuser`; the spec points them at a file it
# holds open (the real path, #1661).
lock_files="${APT_LOCK_FILES:-/var/lib/dpkg/lock-frontend /var/lib/apt/lists/lock}"

# shellcheck disable=SC2086 # $as_root is "sudo" or empty on purpose
printf '%s\n' \
  'Acquire::http::Timeout "30";' \
  'Acquire::https::Timeout "30";' \
  'Acquire::Retries "3";' \
  'DPkg::Lock::Timeout "120";' | $as_root tee "$apt_conf" >/dev/null || exit 1

# True while an apt-get, apt or dpkg process exists. APT_BUSY_CMD replaces the
# check in the spec.
apt_busy() {
  if [ -n "${APT_BUSY_CMD:-}" ]; then
    bash -c "$APT_BUSY_CMD"
  else
    # The lock probe covers the gap between `apt-get update` and
    # `apt-get install` inside Playwright's one root `sh -c`, when no apt
    # process exists but the orphan is about to take the lock again.
    pgrep -x 'apt-get|apt|dpkg' ||
      $as_root fuser $lock_files
  fi
}

# Waits until no apt-get/dpkg process is left, at most $settle_timeout
# seconds. Returns 0 when apt is idle, 1 when the bound was reached.
wait_for_apt_idle() {
  local waited=0
  while apt_busy >/dev/null 2>&1; do
    if [ "$waited" -ge "$settle_timeout" ] || [ "$SECONDS" -ge "$total_budget" ]; then
      echo "apt was still busy after ${settle_timeout}s."
      return 1
    fi
    if [ "$waited" -eq 0 ]; then
      echo "An apt-get from the previous attempt is still running; waiting for it."
    fi
    sleep "$poll_interval"
    waited=$((waited + poll_interval))
  done
  return 0
}

attempt=1
while [ "$attempt" -le "$attempts" ]; do
  remaining=$((total_budget - SECONDS))
  if [ "$remaining" -le 0 ]; then
    echo "The ${total_budget}s budget for this step is spent."
    break
  fi
  this_timeout=$attempt_timeout
  [ "$remaining" -lt "$this_timeout" ] && this_timeout=$remaining
  if "$timeout_cmd" "$this_timeout" bash -c "$install_cmd"; then
    exit 0
  fi
  echo "OS dependency install attempt ${attempt} failed."
  if [ "$attempt" -lt "$attempts" ]; then
    wait_for_apt_idle || true
    echo "Retrying in ${retry_sleep}s..."
    sleep "$retry_sleep"
  fi
  attempt=$((attempt + 1))
done
exit 1

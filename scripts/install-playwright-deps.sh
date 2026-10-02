#!/usr/bin/env bash
# Installs the OS libraries Playwright's Chromium links against, with a
# bounded, retryable apt (#1655). One script for the three CI jobs that need
# it (`site` and `app` in ci.yml, gcp-dashboard-ci.yml), so the fix cannot
# drift between them. Run it from the directory that holds the Playwright
# dependency (`bunx playwright` resolves the version there).
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
# Every knob below can be overridden by an environment variable; the spec in
# scripts/install-playwright-deps.test.ts uses that to run the loop against
# fake commands.
set -u

install_cmd="${INSTALL_CMD:-bunx playwright install-deps chromium}"
attempts="${ATTEMPTS:-3}"
attempt_timeout="${ATTEMPT_TIMEOUT:-180}"
retry_sleep="${RETRY_SLEEP:-10}"
settle_timeout="${SETTLE_TIMEOUT:-420}"
poll_interval="${POLL_INTERVAL:-5}"
apt_conf="${APT_CONF:-/etc/apt/apt.conf.d/99-ci-bounds}"
timeout_cmd="${TIMEOUT_CMD:-timeout}"
as_root="${AS_ROOT-sudo}"

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
    pgrep -x 'apt-get|apt|dpkg'
  fi
}

# Waits until no apt-get/dpkg process is left, at most $settle_timeout
# seconds. Returns 0 when apt is idle, 1 when the bound was reached.
wait_for_apt_idle() {
  local waited=0
  while apt_busy >/dev/null 2>&1; do
    if [ "$waited" -ge "$settle_timeout" ]; then
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
  if "$timeout_cmd" "$attempt_timeout" bash -c "$install_cmd"; then
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

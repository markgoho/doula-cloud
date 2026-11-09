/** Real log excerpts of failed runs, trimmed to the lines before the `##[error]`. */

// Run 42571956473, job 129011109650 (`format`): setup-bun's own retry
// gave up after about 35 seconds of 504s from GitHub Releases.
export const BUN_504 = [
  '2026-11-09T06:15:50.7015361Z ##[group]Run oven-sh/setup-bun@v2',
  '2026-11-09T06:15:50.7017702Z   no-cache: false',
  '2026-11-09T06:15:50.7029383Z ##[endgroup]',
  '2026-11-09T06:15:51.2416361Z Downloading a new version of Bun: https://github.com/oven-sh/bun/releases/download/bun-v1.4.4/bun-linux-x64.zip',
  '2026-11-09T06:15:51.3402189Z Unexpected HTTP response: 504',
  '2026-11-09T06:15:51.3402749Z Waiting 17 seconds before trying again',
  '2026-11-09T06:16:08.4370783Z Unexpected HTTP response: 504',
  '2026-11-09T06:16:08.4371946Z Waiting 17 seconds before trying again',
  '2026-11-09T06:16:25.5636017Z ##[error]Error: Unexpected HTTP response: 504',
].join('\n');

// Run 42566851424, job 128995101516 (`plan-stripe-sandbox`).
export const TERRAFORM_504 = [
  '2026-11-09T05:22:08.9651905Z ',
  '2026-11-09T05:22:08.9652666Z Error: Failed to install provider',
  '2026-11-09T05:22:08.9653253Z Error while installing stripe/stripe v0.3.0: could not query provider',
  '2026-11-09T05:22:08.9653844Z registry for registry.terraform.io/stripe/stripe: failed to retrieve',
  '2026-11-09T05:22:08.9654527Z cryptographic signature for provider: the request failed after 2 attempts,',
  '2026-11-09T05:22:08.9655134Z please try again later: 504 Gateway Timeout returned from github.com',
  '2026-11-09T05:22:08.9686667Z ##[error]Process completed with exit code 1.',
].join('\n');

// Run 42534349738, job 128895256448 (`api`): a real failure of the code.
export const COVCHECK_FAILED = [
  '2026-11-08T23:23:40.5256924Z ok  \tdoula-cloud/api/internal/staffauth\t86.517s\tcoverage: 81.8% of statements',
  '2026-11-08T23:23:41.8714615Z ##[group]Run go run ./tools/covcheck -profile=coverage.out -module=doula-cloud/api -skip=doula-cloud/api/tools/',
  '2026-11-08T23:23:42.0209908Z covcheck: uncovered lines with no coverage:ignore justification:',
  '2026-11-08T23:23:42.0211004Z   doula-cloud/api/internal/visit/create.go:102-103',
  '2026-11-08T23:23:42.0212729Z exit status 1',
  '2026-11-08T23:23:42.0244891Z ##[error]Process completed with exit code 1.',
].join('\n');

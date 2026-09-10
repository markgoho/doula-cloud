import { execFileSync, spawn } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { createConnection } from 'node:net';
import { existsSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
	E2E_API_HOST,
	E2E_API_PORT,
	E2E_EMULATOR_HOST,
	E2E_EMULATOR_PORT,
	DB_HOST,
	DB_PORT,
	GCS_HOST,
	GCS_PORT,
	MAILBOX_HOST,
	MAILBOX_PORT,
	PORT_OFFSET,
	E2E_COMPOSE_FILE,
	e2eComposeProject
} from './ports';

// The fake-gcs-server in compose.e2e.yaml, and the one bucket the BFF is
// pointed at. Both halves matter to the Go storage SDK: it reads
// STORAGE_EMULATOR_HOST as a bare host:port (see the SDK's own doc.go
// example) and builds an http endpoint from it, and it will not create a
// missing bucket on first write -- seedGCSBucket below does that.
const GCS_BUCKET = 'doula-cloud-e2e-attachments';
// Every compose invocation below is scoped to this project name so two
// worktrees' stacks don't collide on container names -- compose defaults
// the project name to the compose dir's basename ("app" everywhere), which
// would otherwise be shared. The name is built in e2e/ports.ts, beside the
// PORT_OFFSET it is derived from, because `.claude/hooks/e2e-stack-reap.ts`
// has to recognize these names to find the stack a killed session left
// running and would go silently inert if the two ever drifted.
const COMPOSE_PROJECT = e2eComposeProject(PORT_OFFSET);
const COMPOSE_ARGS = ['compose', '-p', COMPOSE_PROJECT, '-f', E2E_COMPOSE_FILE];
// compose.e2e.yaml reads these for its host port bindings and the gcs
// service's -external-url; unset (offset 0) falls back to the compose
// file's own defaults, so CI and the main checkout are unaffected.
const COMPOSE_ENV = {
	...process.env,
	...(PORT_OFFSET && { DB_HOST_PORT: String(DB_PORT), GCS_HOST_PORT: String(GCS_PORT) })
};
// Distinguishes this worktree's host-process pidfiles/binary from every
// other worktree's -- see e2e/ports.ts for why PORT_OFFSET is the stable
// per-worktree key. Suffix omitted at offset 0 so the main checkout and CI
// keep today's exact filenames.
const PIDFILE_SUFFIX = PORT_OFFSET ? `-${PORT_OFFSET}` : '';

// firebase.json has no CLI flag for the auth emulator's port -- only
// --config <path>. At offset 0 (main checkout, CI) startEmulator passes no
// --config at all, so firebase-tools resolves the committed firebase.json
// exactly as it does today. A real worktree gets its own copy, port
// rewritten, written next to it and gitignored (see .gitignore).
// import.meta.dir is Bun-only; Playwright's own CLI (node_modules/.bin/playwright)
// runs under Node, and it's the one that loads this file transitively via
// global-setup.ts -- import.meta.url -> fileURLToPath is the portable way
// to get this file's own directory under both runtimes.
const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const FIREBASE_CONFIG_PATH = path.join(REPO_ROOT, 'firebase.json');
const WORKTREE_FIREBASE_CONFIG_PATH = path.join(REPO_ROOT, '.firebase.e2e.json');

function emulatorConfigPath(): string | undefined {
	if (!PORT_OFFSET) return undefined;
	const config = JSON.parse(readFileSync(FIREBASE_CONFIG_PATH, 'utf8'));
	config.emulators.auth.port = E2E_EMULATOR_PORT;
	writeFileSync(WORKTREE_FIREBASE_CONFIG_PATH, JSON.stringify(config, undefined, 2));
	return WORKTREE_FIREBASE_CONFIG_PATH;
}
const READY_TIMEOUT_MS = 60_000;
// `db` and `gcs` (compose.e2e.yaml) just pull/start pinned
// postgres:16-alpine and fake-gcs-server images -- no build involved --
// so they don't need the long budget a Go image build used to require
// here.
const DB_UP_TIMEOUT_MS = 60_000;
// `go run ./cmd/migrate` and `go build` (below) both compile from
// scratch on a cold module/build cache; actions/setup-go's cache (see
// ci.yml) keeps that close to free on an unchanged api/, but give both a
// generous budget for a cold cache or a real api/ change.
const MIGRATE_TIMEOUT_MS = 90_000;
const BUILD_TIMEOUT_MS = 120_000;

// `docker compose` and `podman compose` share the same v2 CLI syntax, so
// this doubles as the binary to exec for the db-only compose stack
// (compose.e2e.yaml) and for the `compose exec` calls below that run SQL
// against it. Local dev defaults to Podman (see docs/testing.md); CI sets
// CONTAINER_ENGINE=docker, since GH-hosted runners ship Docker natively
// with no rootless-socket setup and no docker-compose-as-external
// -provider translation layer required.
const CONTAINER_ENGINE = process.env.CONTAINER_ENGINE ?? 'podman';

// The Firebase Auth emulator (see firebase.json) and the Go BFF (see
// startAPI below) are both plain host processes now -- Postgres is the
// only piece still worth containerizing (see compose.e2e.yaml). Shared
// pidfile pattern for both, so `bun run dev:full` (scripts/dev-full.ts)
// gets the same clean-teardown behavior as the Playwright e2e run
// (global-setup.ts/global-teardown.ts).
const EMULATOR_PIDFILE = path.join(tmpdir(), `doula-cloud-e2e-firebase-emulator${PIDFILE_SUFFIX}.pid`);
const API_PIDFILE = path.join(tmpdir(), `doula-cloud-e2e-api${PIDFILE_SUFFIX}.pid`);
const API_BINARY_PATH = path.join(tmpdir(), `doula-cloud-e2e-api${PIDFILE_SUFFIX}`);
const MAILBOX_PIDFILE = path.join(tmpdir(), `doula-cloud-e2e-mailbox${PIDFILE_SUFFIX}.pid`);
const MAILBOX_SCRIPT = path.join(path.dirname(fileURLToPath(import.meta.url)), 'mailbox.ts');

// The sandbox mail settings (#764). These are set explicitly rather than
// inherited, and that is the point: `app/.env.local` carries a real
// Mailgun key and the account's sandbox domain for interactive dev, bun
// loads it before this file runs, and without these four lines a local
// e2e run or a simulation run would post real mail to real Mailgun.
// MAILGUN_API_BASE redirects every one of the eleven mail kinds to
// e2e/mailbox.ts instead; MAILGUN_DOMAIN is what the From/Reply-To
// identities in api/main.go are built from, so it is the domain every
// persona's address sits under.
const MAILGUN_SIM_DOMAIN = 'sim.doula.cloud';
const MAILGUN_WEBHOOK_SIGNING_KEY = 'e2e-mailgun-signing-key';
// The shared secret every `process-*` outbox endpoint checks
// (X-Internal-Secret, api/internal/outbox/handler.go). Empty refuses
// every call, so a stack that never set it could not drain an outbox at
// all -- which is how mail stayed unobservable here until now.
const NOTIFICATION_WORKER_SECRET = 'e2e-worker-secret';

export const MAILBOX_URL = `http://${MAILBOX_HOST}:${MAILBOX_PORT}`;
export const MAILBOX_DOMAIN = MAILGUN_SIM_DOMAIN;
export const WORKER_SECRET = NOTIFICATION_WORKER_SECRET;

// Brings up the whole e2e stack: the Firebase Auth emulator, the db-only
// compose stack (see compose.e2e.yaml), the goose migrations and the
// app_e2e login role against it, and finally the Go BFF -- all but
// Postgres running as host processes reaching each other over loopback.
// Sequenced by hand (db, then migrate, then the role, then the BFF)
// because that used to be compose's own `depends_on` chain; each step
// still needs the one before it to have actually finished.
//
// appOrigin is the front-end origin this run's caller will actually make
// requests from -- DEV_SERVER_ORIGIN for `bun run dev:full`,
// PREVIEW_SERVER_ORIGIN for the Playwright stack (see ports.ts) -- so the
// BFF's csrf.Wrap (api/internal/csrf) is configured for the one origin
// that's really going to call it, not a hardcoded union of both.
export async function startStack(appOrigin: string) {
	await startEmulator();
	await startDatabase();
	installSimClock();
	runMigrations();
	seedAppE2ERole();
	grantSimClock();
	await seedGCSBucket();
	await startMailbox();
	await startAPI(appOrigin);
}

// The sandbox mailbox (e2e/mailbox.ts), started before the BFF for the
// same reason the database is: the BFF is pointed at it at boot. A plain
// Bun host process, tracked by pidfile exactly like the emulator.
async function startMailbox() {
	killPidfile(MAILBOX_PIDFILE);
	const child = spawn('bun', [MAILBOX_SCRIPT], {
		stdio: 'inherit',
		detached: true,
		env: { ...process.env, MAILGUN_WEBHOOK_SIGNING_KEY }
	});
	child.unref();
	if (child.pid) {
		writeFileSync(MAILBOX_PIDFILE, String(child.pid));
	}
	await waitForPort(MAILBOX_HOST, MAILBOX_PORT, READY_TIMEOUT_MS);
}

// Deliberately doesn't use `up --wait` for `db`: under CI's rootless
// Podman (when CONTAINER_ENGINE=podman) with docker-compose-as-provider,
// the container starts fine but its healthcheck never reports "healthy",
// so --wait blocks forever (confirmed on trunk -- see git log for this
// file). Polling the host-exposed port directly sidesteps that.
async function startDatabase() {
	try {
		// stdio left as the default (captured), not 'inherit': 'inherit'
		// hands the child the parent's own file descriptors, so Node never
		// fills in error.stdout/error.stderr on a non-zero exit -- the exit
		// status this then throws is all a caller ever sees (#782). Capturing
		// costs nothing here (compose up -d exits almost immediately) and
		// lets the catch below put podman-compose's own message on the
		// console before re-throwing.
		const output = execFileSync(CONTAINER_ENGINE, [...COMPOSE_ARGS, 'up', '-d'], {
			timeout: DB_UP_TIMEOUT_MS,
			env: COMPOSE_ENV
		});
		process.stdout.write(output);
	} catch (error) {
		if (error instanceof Error) {
			const { stdout, stderr } = error as Error & { stdout?: Buffer; stderr?: Buffer };
			if (stdout?.length) process.stdout.write(stdout);
			if (stderr?.length) process.stderr.write(stderr);
		}
		throw error;
	}
	await waitForPort(DB_HOST, DB_PORT, READY_TIMEOUT_MS);
	await waitForPort(GCS_HOST, GCS_PORT, READY_TIMEOUT_MS);
}

// Creates the one bucket the BFF writes to, the same way seedAppE2ERole
// creates the one role it logs in as -- a compose service comes up empty,
// and neither the SDK nor the BFF creates its own container. Idempotent
// by way of fake-gcs-server answering 409 for a bucket that already
// exists, which a re-run against a still-warm stack will hit.
async function seedGCSBucket() {
	const response = await fetch(`http://${GCS_HOST}:${GCS_PORT}/storage/v1/b?project=doula-cloud`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name: GCS_BUCKET })
	});
	if (!response.ok && response.status !== 409) {
		throw new Error(`could not create the ${GCS_BUCKET} bucket: ${response.status} ${await response.text()}`);
	}
}

// Installs simulated time (#762, #778) into the compose `db` as the
// `app` superuser, before a single migration runs -- DEFAULT now() binds
// to a function at CREATE TABLE time, so installed any later, every
// existing default would stay on real time. Idempotent: a resumed run
// against a kept volume calls this again and it's a no-op.
function installSimClock() {
	execFileSync('go', ['run', './cmd/simclock', 'install', 'app'], {
		stdio: 'inherit',
		cwd: '../api',
		timeout: MIGRATE_TIMEOUT_MS,
		env: {
			...process.env,
			DATABASE_URL: `postgres://app:app@${DB_HOST}:${DB_PORT}/app?sslmode=disable`
		}
	});
}

// Points app_e2e's search_path at the sim schema installSimClock
// created, and grants it the USAGE and SELECT sim.now() needs to run
// under a non-superuser role. Must run after seedAppE2ERole -- the role
// doesn't exist before that.
function grantSimClock() {
	execFileSync('go', ['run', './cmd/simclock', 'grant', 'app_e2e'], {
		stdio: 'inherit',
		cwd: '../api',
		timeout: MIGRATE_TIMEOUT_MS,
		env: {
			...process.env,
			DATABASE_URL: `postgres://app:app@${DB_HOST}:${DB_PORT}/app?sslmode=disable`
		}
	});
}

// Runs api/cmd/migrate as a host process against the compose `db`,
// connecting as the `app` superuser (the same role compose's old
// `migrate` service used). cmd/migrate retries its own connection for up
// to 30s (see its connectTimeout), so there's no separate db-readiness
// wait needed here beyond startDatabase's own port check.
function runMigrations() {
	execFileSync('go', ['run', './cmd/migrate'], {
		stdio: 'inherit',
		cwd: '../api',
		timeout: MIGRATE_TIMEOUT_MS,
		env: {
			...process.env,
			DATABASE_URL: `postgres://app:app@${DB_HOST}:${DB_PORT}/app?sslmode=disable`
		}
	});
}

// Runs one SQL statement against the compose `db` via `compose exec` +
// psql, rather than adding a new Postgres client dependency to app/ for
// the handful of one-shot statements the e2e stack needs (provisioning
// app_e2e below, and seedClientPortalUser's insert).
//
// Values are inlined into the -c string (escaped, not passed via psql's
// own -v/:'var' substitution): podman-compose's `exec` passthrough
// doesn't forward -v flags to the exec'd process intact (confirmed
// empirically -- the SQL psql received still had the literal `:'uid'`
// text in it, unsubstituted), so :'var' interpolation never fires. Every
// caller of sqlLiteral in this file controls its own input (this test
// run's prior API/emulator calls, or a hardcoded role name), not
// external input.
function execSQL(sql: string) {
	execFileSync(
		CONTAINER_ENGINE,
		[...COMPOSE_ARGS, 'exec', '-T', 'db', 'psql', '-U', 'app', '-d', 'app', '-v', 'ON_ERROR_STOP=1', '-c', sql],
		{ stdio: 'inherit', timeout: 30_000, env: COMPOSE_ENV }
	);
}

function sqlLiteral(value: string): string {
	return `'${value.replaceAll('\'', "''")}'`;
}

// Creates the low-privilege LOGIN role the RLS policies in db/migrations
// apply to. Migration 00002 creates the app_runtime *group* role, but
// nothing can log in as it until a member role exists -- a real deploy
// would provision that via Cloud SQL IAM instead of a hardcoded
// password. Used to be compose.e2e.yaml's own `seed-role` service; now
// runs the identical SQL via execSQL once runMigrations has finished.
function seedAppE2ERole() {
	execSQL(
		"DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'app_e2e') THEN " +
			"CREATE ROLE app_e2e LOGIN PASSWORD 'app_e2e' IN ROLE app_runtime; END IF; END $$;"
	);
}

// Links a fresh Portal Account to clientID via client_portal_users, with
// signInAddress as its sign-in address -- the address a spec's own
// signInPortalClient (portalClient.ts) requests a magic link for. Nothing
// in the BFF creates this row (see #53: v1 has no Client-portal-
// provisioning endpoint, staff-side or otherwise -- Staff creates a
// Client + Engagement per #52, but linking a Client to portal login
// credentials isn't spec'd anywhere yet), so the e2e test that proves
// login->landing works has to seed it itself.
//
// The identifier is minted here rather than taken from a caller (#617
// removed the Identity Platform uid that used to fill this role): a
// portal_-prefixed random id, the same shape portalaccount.NewIdentifier
// mints in the real accept-invite path.
export function seedClientPortalUser(signInAddress: string, clientID: string) {
	const identifier = `portal_${randomUUID()}`;
	execSQL(
		`INSERT INTO portal_accounts (identifier, sign_in_address) VALUES (${sqlLiteral(identifier)}, ${sqlLiteral(signInAddress)})`
	);
	execSQL(`INSERT INTO client_portal_users (identity_uid, client_id) VALUES (${sqlLiteral(identifier)}, ${sqlLiteral(clientID)})`);
}

// Seeds an Engagement directly: #397 decoupled Client creation from
// Engagement creation (ADR-0017 -- an Engagement now comes from a
// separate Engagement Request, not built yet), so POST .../clients no
// longer returns an engagementId for e2e specs that need one to reach an
// Engagement-scoped screen or endpoint.
export function seedEngagement(clientId: string, practiceId: string, status = 'intake', kind = 'birth'): string {
	const engagementId = randomUUID();
	execSQL(
		`INSERT INTO engagements (id, client_id, practice_id, status, kind) VALUES (${sqlLiteral(engagementId)}, ${sqlLiteral(clientId)}, ${sqlLiteral(practiceId)}, ${sqlLiteral(status)}, ${sqlLiteral(kind)})`
	);
	return engagementId;
}

/**
 * Clears every rate-limit counter the BFF is holding (#1138), so a
 * repeated batch can keep spending budgets a person never would.
 *
 * All of them, not one endpoint's: a repeated batch reaches more than one
 * ceiling, so clearing an endpoint at a time only moves the wall rather
 * than removing it. retryPastRateLimit (rateLimit.ts) is the one place
 * that argument is written out, along with which budgets this suite spends
 * and why the recovery is reactive; this function is only its hands.
 * `rate_limit_buckets` holds nothing but counters
 * (`api/internal/ratelimit`), so emptying it leaves the stack in the state
 * it was in before it had served any traffic.
 *
 * **This is not a switch anything can turn on.** It is a DELETE issued by
 * psql inside the e2e stack's own compose database, the same seam
 * seedEngagement and seedPracticeRate already write rows through. There
 * is no environment variable, no header and no product code path, so
 * there is nothing here that could be enabled against a deployed BFF --
 * the mechanism *is* holding the database. The limiter itself is
 * untouched and still refuses a genuine burst;
 * `TestSignupRefusesAGenuineBurst` (api/internal/staffauth) asserts that
 * refusal at the mounted route so this escape hatch cannot quietly become
 * a disabled limiter. 00060 grants app_runtime only SELECT, INSERT and
 * UPDATE on this table, so the role the BFF connects as could not run
 * this statement even if some future handler tried to: clearing a counter
 * is the table owner's to do, and in a deploy that is not the BFF.
 *
 * Two workers clearing at once is harmless -- clearing only ever raises
 * what is allowed -- and no e2e spec asserts a 429 on any endpoint, so
 * there is no assertion for a clear to race.
 */
export function resetRateLimits() {
	execSQL('DELETE FROM rate_limit_buckets');
}

/**
 * Seeds a practice_rates row directly (#966/#967): PostContractHandler
 * refuses to create a Contract at all when the Practice has no rate set
 * for the Engagement's kind, so every e2e spec that creates a Contract
 * through the API needs one seeded first, the same way seedEngagement
 * bypasses the API for its own row.
 */
export function seedPracticeRate(practiceId: string, kind = 'birth', amountCents = 15_000): void {
	execSQL(
		`INSERT INTO practice_rates (practice_id, kind, amount_cents) VALUES (${sqlLiteral(practiceId)}, ${sqlLiteral(kind)}, ${amountCents})`
	);
}

// Seeds a Message whose attachment metadata points at an object never
// Put to the store -- #305's not-found handling only differs from a
// genuine storage failure once the DB row and the store disagree, which
// the real upload path (Put then the row write) never leaves behind on
// its own. Direct SQL, like seedEngagement, is what makes that
// disagreement reachable at all.
export function seedMessageWithMissingAttachment(engagementId: string, staffId: string, filename: string): string {
	const messageId = randomUUID();
	const objectPath = `messages/${engagementId}/${messageId}`;
	execSQL(
		`INSERT INTO messages (id, engagement_id, sender_type, sender_id, attachment_object_path, attachment_content_type, attachment_byte_size, attachment_filename) VALUES (${sqlLiteral(messageId)}, ${sqlLiteral(engagementId)}, 'staff', ${sqlLiteral(staffId)}, ${sqlLiteral(objectPath)}, 'image/png', 1, ${sqlLiteral(filename)})`
	);
	return messageId;
}

// Seeds a *pending* Engagement Request directly, the same way
// seedEngagement seeds an Engagement directly: the real
// POST .../engagement-requests endpoint collapses into an immediate
// approval whenever the requester already holds approval authority
// (RequestHandler, ADR-0017) -- which the Owner/Admin session every e2e
// spec signs in as always does -- so driving that endpoint can never
// leave a Request sitting in 'pending'. requestedBy is the requesting
// Staff member's id (not a session header), matching engagement_requests'
// own requested_by column.
export function seedEngagementRequest(
	clientId: string,
	practiceId: string,
	requestedBy: string,
	kind = 'birth'
): string {
	const requestId = randomUUID();
	execSQL(
		`INSERT INTO engagement_requests (id, practice_id, client_id, kind, requested_by) VALUES (${sqlLiteral(requestId)}, ${sqlLiteral(practiceId)}, ${sqlLiteral(clientId)}, ${sqlLiteral(kind)}, ${sqlLiteral(requestedBy)})`
	);
	return requestId;
}

// `-t -A` (tuples-only, unaligned) is what keeps psql's output to just
// the value, with no header or border for a caller to strip.
function querySQLValue(sql: string): string {
	const output = execFileSync(
		CONTAINER_ENGINE,
		[...COMPOSE_ARGS, 'exec', '-T', 'db', 'psql', '-U', 'app', '-d', 'app', '-t', '-A', '-v', 'ON_ERROR_STOP=1', '-c', sql],
		{ timeout: 30_000, env: COMPOSE_ENV }
	);
	return output.toString('utf8').trim();
}

// The accept link staffinvite's Compose writes into the invitation mail
// (api/internal/staffinvite/outbox.go): the token is the whole query
// string, and the body is plain text -- ADR-0030 rules out any tracking
// wrapper, so the link in the mail is the link that was sent.
const ACCEPT_INVITE_LINK = /\/accept-invite\?token=(\S+)/g;

// Reads one invitation's token out of the sandbox mailbox
// (e2e/mailbox.ts), the second place it lives once the outbox row has
// been drained.
//
// A mailbox is keyed on the address, and an address can hold more than
// one live invitation -- mfa-required.e2e.ts invites one identity to two
// Practices -- so "the newest link in this inbox" is not the same
// question as "this invitation's link". digest settles it: it is
// practice_invitations.token_digest, the SHA-256 hex of the token this
// invitation was minted with (staffauth.TokenDigest), so the candidate
// whose own digest matches is the right one and no other candidate can
// be mistaken for it.
async function readInviteTokenFromMailbox(address: string, digest: string): Promise<string> {
	const inbox = await fetch(`${MAILBOX_URL}/api/messages?to=${encodeURIComponent(address)}`);
	if (!inbox.ok) {
		throw new Error(`stack: reading ${address}'s sandbox mailbox failed: ${inbox.status}`);
	}
	const messages = (await inbox.json()) as { text: string }[];
	for (const message of messages) {
		for (const [, candidate] of message.text.matchAll(ACCEPT_INVITE_LINK)) {
			if (createHash('sha256').update(candidate).digest('hex') === digest) {
				return candidate;
			}
		}
	}
	return '';
}

// Reads the plaintext token for invitationId (#525). InviteResponse
// deliberately never carries it (#316), so a spec that walks the accept
// flow has to read it out of the stack, and there are exactly two places
// it is ever readable: the pending staff_invite_outbox row before the
// invitation is mailed, and the mail itself afterwards.
//
// Both, not just the first (#827). The outbox row is not durable for the
// length of a spec: `process-staff-invite-outbox` claims every due
// pending row, whatever invitation it belongs to (staffinvite's
// claimQuery filters on status and next_attempt_at and nothing else), and
// mail-delivery.e2e.ts drains that outbox as the very thing it is about.
// Playwright runs spec files in parallel against one shared stack, so
// that drain lands mid-invite for whatever spec is running beside it,
// sends the mail, and clears invite_token on the now-`sent` row -- which
// left this read finding nothing, intermittently, for a reason having
// nothing to do with the spec that failed.
//
// The mailbox fallback closes that hole rather than papering over it with
// a retry, which could not help: the row is gone for good, not late.
// outbox.MailWorker sends before it marks (mailworker.go's
// claim-scan-compose-send-mark loop) and marks inside the drain's own
// transaction, so a row that went terminal *because it was drained* had
// its mail delivered first, every time.
//
// MailWorker does have two branches that go terminal with nothing sent --
// staffinvite's Compose returns ErrAlreadyDone for an Invitation already
// resolved or expired, and mail.ErrSuppressed dead-letters a blocked
// address -- but neither can be true of an Invitation a spec is in the
// middle of accepting: it is pending and unexpired by construction, and
// InviteHandler refuses a suppressed address before any row exists
// (#861). If one ever were, this throws naming both places it looked
// rather than returning an empty string a caller pastes into a URL.
export async function readStaffInviteToken(invitationId: string): Promise<string> {
	const pending = querySQLValue(
		`SELECT invite_token FROM staff_invite_outbox WHERE invitation_id = ${sqlLiteral(invitationId)} AND status = 'pending'`
	);
	if (pending !== '') {
		return pending;
	}

	// psql's unaligned output separates columns with `|`, and neither an
	// address nor a hex digest can contain one.
	const invitation = querySQLValue(
		`SELECT address, token_digest FROM practice_invitations WHERE id = ${sqlLiteral(invitationId)}`
	);
	if (invitation === '') {
		throw new Error(`stack: no practice_invitations row for invitation ${invitationId}`);
	}
	const [address, digest] = invitation.split('|', 2);

	const mailed = await readInviteTokenFromMailbox(address, digest);
	if (mailed === '') {
		throw new Error(
			`stack: no invite token for invitation ${invitationId}: no pending staff_invite_outbox row, and no accept link matching its token digest in ${address}'s sandbox mailbox`
		);
	}
	return mailed;
}

// A run's own interval literal, e.g. "1 day" or "90 minutes" -- the shape
// Postgres's interval input accepts. Guarded the same way roleName is in
// api/internal/simclock: every caller here passes a value it built itself
// from a number and a fixed unit (see simulation/clock.ts), never
// anything read off a request, but rejecting anything else keeps that
// true by construction rather than by convention.
const intervalLiteral = /^\d+ (seconds|minutes|hours|days)$/;

// Advances sim.offset_row.delta by amount (#762, #778) -- the one write
// every simulation jump makes. Additive, so two jumps compose: advancing
// "1 day" twice is the same as advancing "2 days" once.
export function advanceOffset(amount: string) {
	if (!intervalLiteral.test(amount)) {
		throw new Error(`stack: invalid interval literal ${JSON.stringify(amount)}`);
	}
	execSQL(`UPDATE sim.offset_row SET delta = delta + interval ${sqlLiteral(amount)}`);
}

// Reads the database's current simulated time, i.e. what sim.now() itself
// would return right now -- the harness's own read of the clock it moves,
// never derived by re-summing the amounts passed to advanceOffset.
export function readSimulatedNow(): string {
	return querySQLValue('SELECT sim.now()');
}

// stopStack tears the stack down. keepVolume leaves the Postgres volume,
// the fake GCS bucket and every host-process pidfile's target already
// dead -- the shape a resumed run needs (#763): the world (rows,
// sim.offset_row, Stripe's test clocks, which live at Stripe) survives,
// and only the host processes, which hold no state of their own, are
// killed and restarted by a later startStack.
export function stopStack({ keepVolume = false }: { keepVolume?: boolean } = {}) {
	killPidfile(API_PIDFILE);
	rmSync(API_BINARY_PATH, { force: true });
	killPidfile(MAILBOX_PIDFILE);
	killPidfile(EMULATOR_PIDFILE);
	execFileSync(CONTAINER_ENGINE, [...COMPOSE_ARGS, 'down', ...(keepVolume ? [] : ['-v'])], {
		stdio: 'inherit',
		timeout: 60_000,
		env: COMPOSE_ENV
	});
}

async function startEmulator() {
	// A crashed previous run can leave the emulator holding its port.
	// Clear it out before starting a fresh one instead of failing to bind.
	killPidfile(EMULATOR_PIDFILE);

	// firebase.json sets emulators.auth.host to 0.0.0.0 rather than the
	// Firebase CLI's 127.0.0.1 default. That used to matter for a second
	// reason beyond "any host process can reach it" -- the old `api`
	// container needed a route to a host-bound service, which a
	// loopback-only bind doesn't give a container (this was the actual
	// cause of a "401 invalid token" e2e failure that persisted across
	// both Podman and Docker in CI; see git log for this file). Now that
	// the BFF is a host process too (startAPI below), that reason is
	// gone, but the 0.0.0.0 bind is harmless to leave as-is.

	const configPath = emulatorConfigPath();
	const arguments_ = ['firebase-tools', 'emulators:start', '--only', 'auth', '--project', 'doula-cloud'];
	if (configPath) arguments_.push('--config', configPath);

	const child = spawn('bunx', arguments_, { stdio: 'ignore', detached: true });
	child.unref();
	if (child.pid) {
		writeFileSync(EMULATOR_PIDFILE, String(child.pid));
	}

	await waitForPort(E2E_EMULATOR_HOST, E2E_EMULATOR_PORT, READY_TIMEOUT_MS);
}

// Builds the BFF once, then runs it as a host process -- `go build` up
// front (rather than `go run`) gives startAPI a stable binary path and a
// direct child PID to track, so teardown (stopStack) can kill it cleanly
// the same way it kills the emulator, instead of having to guess at a
// `go run`-spawned child's PID.
//
// Env mirrors what compose.e2e.yaml's old `api` service set, with
// loopback addresses in place of compose service names/host-gateway
// hostnames (a host process reaches Postgres, the emulator, and its own
// listener over 127.0.0.1 directly -- no container-to-host routing to
// work around). STORAGE_EMULATOR_HOST points at the compose `gcs`
// service: setting it at all is what makes the GCS SDK skip Application
// Default Credentials discovery at startup (which otherwise blocks on a
// real metadata-server probe with no route to succeed here, delaying
// main()'s listener past the point Playwright starts hitting it), and
// pointing it somewhere real is what makes the object store *work*. It
// used to be `storage-emulator-disabled.invalid:1` on the grounds that no
// e2e test touches the attachment endpoints (#60) -- but Contract signing
// puts the signed PDF in the store before it writes the status
// (api/internal/contracts/sign.go), so an unreachable store made every
// signing step, automated or hand-walked, a 500 (#234).
// VAPID_PUBLIC_KEY/VAPID_PRIVATE_KEY are a throwaway
// keypair (generated once via webpush.GenerateVAPIDKeys(), not a real
// secret) purely so push.NewVAPIDPusher constructs cleanly at startup --
// no e2e test registers a real push_subscriptions row through the UI
// (per #61, PushManager.subscribe() doesn't work headless), so
// Pusher.Send is never actually reached, but this keeps main() from ever
// depending on real VAPID keys existing to boot.
async function startAPI(appOrigin: string) {
	killPidfile(API_PIDFILE);

	execFileSync('go', ['build', '-o', API_BINARY_PATH, '.'], {
		stdio: 'inherit',
		cwd: '../api',
		timeout: BUILD_TIMEOUT_MS
	});

	const child = spawn(API_BINARY_PATH, [], {
		// Unlike the emulator, the BFF's own logs are worth seeing inline
		// in CI when an e2e run fails -- 'inherit' rather than 'ignore'.
		stdio: 'inherit',
		detached: true,
		env: {
			...process.env,
			DATABASE_URL: `postgres://app_e2e:app_e2e@${DB_HOST}:${DB_PORT}/app?sslmode=disable`,
			FIREBASE_AUTH_EMULATOR_HOST: `${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`,
			GCP_PROJECT_ID: 'doula-cloud',
			STORAGE_EMULATOR_HOST: `${GCS_HOST}:${GCS_PORT}`,
			GCS_ATTACHMENTS_BUCKET: GCS_BUCKET,
			VAPID_PUBLIC_KEY: 'BEOwHFGQTdwLqgkxPeDzvHQAHjqFkfxMVdO8ONFexrHrD4_43Jvr_XPB5LUA6AAdvnGK1sHeo7WYPwCAOfRI9Ow',
			VAPID_PRIVATE_KEY: 'Vb9fJN9OddK_iRPHqg4We5I2KIcppbZS9_-aoAELXI4',
			VAPID_SUBSCRIBER: 'mailto:e2e@doula-cloud.invalid',
			EXPECTED_ORIGINS: appOrigin,
			// Every Stripe redirect target is built off this (Checkout's
			// success/cancel URLs, the Connect Account Link's
			// return/refresh URLs -- api/internal/billing and
			// api/internal/payments), so it has to be the origin the
			// browser walking this stack is actually on, not a fixed
			// value. Same string as EXPECTED_ORIGINS by default; a
			// tunnelled walk (see docs/environment.md) overrides it via
			// app/.env.local, which bun loads before this file runs.
			APP_BASE_URL: process.env.APP_BASE_URL ?? appOrigin,
			MAILGUN_API_BASE: MAILBOX_URL,
			MAILGUN_API_KEY: 'e2e-mailgun-key',
			MAILGUN_DOMAIN: MAILGUN_SIM_DOMAIN,
			MAILGUN_WEBHOOK_SIGNING_KEY,
			NOTIFICATION_WORKER_SECRET,
			PORT: String(E2E_API_PORT)
		}
	});
	child.unref();
	if (child.pid) {
		writeFileSync(API_PIDFILE, String(child.pid));
	}

	await waitForPort(E2E_API_HOST, E2E_API_PORT, READY_TIMEOUT_MS);
}

// Kills whatever process the given pidfile points at (if any) and
// removes it. Used both to clear a stale pidfile before starting a fresh
// process and to stop a process this module started.
function killPidfile(pidfile: string) {
	if (!existsSync(pidfile)) return;
	const pid = Number(readFileSync(pidfile, 'utf8'));
	try {
		process.kill(pid);
	} catch {
		// Already gone -- nothing to clean up.
	}
	rmSync(pidfile, { force: true });
}

function waitForPort(host: string, port: number, timeoutMs: number): Promise<void> {
	const deadline = Date.now() + timeoutMs;
	return new Promise((resolve, reject) => {
		const attempt = () => {
			const socket = createConnection({ host, port }, () => {
				socket.end();
				resolve();
			});
			socket.on('error', () => {
				socket.destroy();
				if (Date.now() > deadline) {
					reject(new Error(`${host}:${port} not reachable after ${timeoutMs}ms`));
					return;
				}
				setTimeout(attempt, 1000);
			});
		};
		attempt();
	});
}

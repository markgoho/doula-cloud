// The jump-and-drain loop (#762, #765), built for #779 under map #759:
// advance the offset row, drain every outbox, and label the sandbox
// mailbox with the run's own clock -- the three things that have to
// happen together on every jump, in that order, so a nudge queued by the
// advance is drained before the mailbox's arrival order is asked to mean
// anything.
import { E2E_API_HOST, E2E_API_PORT } from '../ports';
import { MAILBOX_URL, advanceOffset, readSimulatedNow } from '../stack';

// api/internal/outbox/drain.go's DrainPath: the one endpoint #794 put
// every registered outbox behind, so this file drains all of them without
// naming or counting a single one -- the ticket's own rule ("do not write
// the number thirteen down anywhere").
const DRAIN_PATH = '/api/internal/outboxes/drain';

export type SimUnit = 'seconds' | 'minutes' | 'hours' | 'days';

export interface JumpAmount {
	value: number;
	unit: SimUnit;
}

export interface JumpResult {
	label: string;
	simulatedNow: string;
	// A jump longer than 12 simulated hours expires every open session,
	// because the Firebase ID-token verifier is hard-wired to real time
	// (calendar.md's jump schedule). The caller re-authenticates whoever
	// is active across a jump this flags.
	crossedReauthThreshold: boolean;
}

const HOURS_PER_UNIT: Record<SimUnit, number> = { seconds: 1 / 3600, minutes: 1 / 60, hours: 1, days: 24 };
const REAUTH_THRESHOLD_HOURS = 12;

function toHours(amount: JumpAmount): number {
	return amount.value * HOURS_PER_UNIT[amount.unit];
}

// Postgres's interval input, e.g. "3 days" -- stack.ts's advanceOffset
// checks this same shape again before it ever reaches SQL.
function toIntervalLiteral(amount: JumpAmount): string {
	return `${amount.value} ${amount.unit}`;
}

async function postJSON(url: string, body: unknown, headers: Record<string, string> = {}) {
	const response = await fetch(url, {
		method: 'POST',
		headers: { 'content-type': 'application/json', ...headers },
		body: JSON.stringify(body)
	});
	if (!response.ok) {
		throw new Error(`jump: ${url} -> ${response.status} ${await response.text()}`);
	}
	return response;
}

// jump advances simulated time by amount and returns once the world is
// caught up: every registered outbox drained (the BFF's own reproduction
// of what Cloud Scheduler does deployed) and the mailbox told what to
// label the messages it receives next. It does not sign anyone back in --
// that is the caller's job, gated on crossedReauthThreshold, because only
// the caller knows which persona sessions are open.
export async function jump(amount: JumpAmount, options: { label: string; workerSecret: string }): Promise<JumpResult> {
	advanceOffset(toIntervalLiteral(amount));

	const drained = await fetch(`http://${E2E_API_HOST}:${E2E_API_PORT}${DRAIN_PATH}`, {
		method: 'POST',
		headers: { 'X-Internal-Secret': options.workerSecret }
	});
	if (!drained.ok) {
		throw new Error(`jump: drain -> ${drained.status} ${await drained.text()}`);
	}

	await postJSON(`${MAILBOX_URL}/api/clock`, { label: options.label });

	return {
		label: options.label,
		simulatedNow: readSimulatedNow(),
		crossedReauthThreshold: toHours(amount) >= REAUTH_THRESHOLD_HOURS
	};
}

import { refusal, type Outcome } from './act.ts';
import { sentAt } from './feedback.ts';

type Instant = string;

export type Refusals = { signature: number; livemode: number; body: number; payload: number; other: number; failed: number };

export type System = {
	stripe: {
		mode: 'test' | 'live' | '';
		platformAccountId: string | null;
		undelivered: { count: number; atLeast: boolean } | null;
	};
	webhooks: { route: string; lastArrivalAt: Instant | null; refusals: Refusals }[];
	eventTypes: { route: string; type: string; lastArrivalAt: Instant | null }[];
	outboxes: {
		name: string;
		pending: number;
		sent: number;
		skipped: number;
		deadLettered: number;
		bounced: number;
		complained: number;
		oldestDueAt: Instant | null;
	}[];
	pilot: {
		gate: { openedAt: Instant | null; openedBy: string | null };
		allowlist: { total: number; items: { address: string; addedAt: Instant; addedBy: string | null }[] };
	};
};

export async function readSystem(fetcher: typeof fetch = fetch): Promise<Outcome<System>> {
	const response = await fetcher('/api/system', { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}

const ROUTE_LABELS: Record<string, string> = {
	stripe_platform: 'Stripe platform',
	stripe_connect: 'Stripe Connect',
	stripe_account: 'Stripe account',
	mailgun: 'Mailgun bounces and complaints'
};

export function routeLabel(route: string): string {
	return ROUTE_LABELS[route] ?? route;
}

export function modeLabel(mode: System['stripe']['mode']): string {
	if (mode === 'test') return 'Test mode';
	if (mode === 'live') return 'Live mode';
	return 'No Stripe key is set';
}

// An outbox is its table's name: portal_invite_outbox reads "Portal invite".
export function outboxLabel(name: string): string {
	const words = name.replace(/_outbox$/, '').replaceAll('_', ' ');
	return (words.charAt(0).toUpperCase() + words.slice(1)).replace(/\bmfa\b/, 'MFA');
}

export function arrivalLabel(instant: Instant | null): string {
	return instant ? sentAt(instant) : 'Nothing yet';
}

export function typeArrivalLabel(instant: Instant | null): string {
	return instant ? sentAt(instant) : 'Not seen yet';
}

export function refusalTotal(refusals: Refusals): number {
	return refusals.signature + refusals.livemode + refusals.body + refusals.payload + refusals.other + refusals.failed;
}

function count(n: number, one: string, many: string): string {
	return `${n} ${n === 1 ? one : many}`;
}

// The kinds that happened, in the order an operator acts on them: "2 signature, 1 livemode".
export function refusalBreakdown(refusals: Refusals): string {
	const kinds: [number, string][] = [
		[refusals.livemode, 'livemode'],
		[refusals.signature, 'signature'],
		[refusals.failed, 'handler failed'],
		[refusals.body, 'unreadable'],
		[refusals.payload, 'bad content'],
		[refusals.other, 'other']
	];
	return kinds
		.filter(([n]) => n > 0)
		.map(([n, kind]) => `${n} ${kind}`)
		.join(', ');
}

export function undeliveredLabel(undelivered: NonNullable<System['stripe']['undelivered']>): string {
	if (undelivered.atLeast) return `${undelivered.count} or more`;
	return count(undelivered.count, 'event', 'events');
}

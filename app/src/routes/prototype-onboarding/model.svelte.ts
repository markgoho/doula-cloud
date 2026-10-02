/*
 * PROTOTYPE -- throwaway, wayfinder ticket #1496. The whole state of the
 * route, in memory: no API and no persistence. A reload starts again.
 */
import { agencyDoulas, samples, type Content, type Owner } from './fixtures.js';

export type Screen = 'signup' | 'overview' | 'name' | 'start' | 'engagement' | 'client';
export type CreditsPlace = 'A' | 'B' | 'C';

export const SCREENS: { key: Screen; label: string }[] = [
	{ key: 'signup', label: '1. Sign up' },
	{ key: 'overview', label: '2. The Practice' },
	{ key: 'name', label: "3. The Client's name" },
	{ key: 'start', label: '4. Start work' },
	{ key: 'engagement', label: '5. The Engagement' },
	{ key: 'client', label: "The Client's record" }
];

export const CREDITS_PLACES: { key: CreditsPlace; name: string }[] = [
	{ key: 'A', name: 'Own paragraph, before the link (as decided)' },
	{ key: 'B', name: 'In the same paragraph as the words' },
	{ key: 'C', name: 'After the link, smaller' }
];

export const NO_DOULA = 'none';
export const SELF = 'self';
const ROUTE = '#/prototype-onboarding';

export interface Account {
	firstName: string;
	lastName: string;
	practiceName: string;
	workState: string;
	timezone: string;
	email: string;
}

export interface Client {
	givenName: string;
	familyName: string;
	preferredName: string;
}

export interface Engagement {
	kind: 'birth' | 'postpartum';
	dueDate: string;
	doula: string;
	note: string;
}

export interface LogEntry {
	record: string;
	detail: string;
}

// Every link on the route stays on the route: `+page.svelte` reads `to`.
export function to(screen: Screen): string {
	return `${ROUTE}?to=${screen}`;
}

// A link the prototype does not follow (a nav section).
export function nowhere(name: string): string {
	return `${ROUTE}?nav=${encodeURIComponent(name)}`;
}

class Prototype {
	screen = $state<Screen>('signup');
	owner = $state<Owner>('solo');
	content = $state<Content>('long');
	creditsPlace = $state<CreditsPlace>('A');
	account = $state<Account>();
	client = $state<Client>();
	engagement = $state<Engagement>();
	balance = $state(3);
	// Presses since the empty Practice (#1516 counts three).
	presses = $state(0);
	log = $state<LogEntry[]>([]);
	// Bumped by "Fill the form", so that a screen can read the sample again.
	fill = $state(0);
	// What the bar says when a link leads out of the prototype.
	note = $state('');
	// Bumped by "Start again", so that the signup form mounts empty.
	run = $state(0);

	sample = $derived(samples[this.content]);
	ownerName = $derived(this.account ? `${this.account.firstName} ${this.account.lastName}` : '');
	clientName = $derived(this.client ? this.client.preferredName || [this.client.givenName, this.client.familyName].filter(Boolean).join(' ') : '');
	// #1596: the employee Doulas at the Practice, the person at the form first.
	doulas = $derived([
		{ value: SELF, label: `${this.ownerName} (you)` },
		...(this.owner === 'agency' ? agencyDoulas[this.content].map((name) => ({ value: name, label: name })) : [])
	]);

	doulaLabel(value: string): string {
		if (value === NO_DOULA) return 'No Doula yet';
		return value === SELF ? this.ownerName : value;
	}

	signUp(account: Account) {
		// #1536: an empty Practice name takes her own name.
		const practiceName = account.practiceName.trim() || `${account.firstName} ${account.lastName}`;
		this.account = { ...account, practiceName };
		this.log.push(
			{ record: 'practices', detail: `"${practiceName}", 3 Credits (the signup bonus)` },
			{ record: 'activity', detail: 'terms_accepted: Terms 2026-10-15, Privacy Policy 2026-10-15, IP address (#1494)' }
		);
		this.go('overview');
	}

	saveClient(client: Client) {
		this.client = client;
		this.presses += 1;
		this.log.push({ record: 'client_events', detail: `created: ${this.clientName}, by ${this.ownerName}. Free.` });
		this.go('start');
	}

	startWork(engagement: Engagement) {
		this.engagement = engagement;
		this.balance -= 1;
		this.presses += 1;
		this.log.push(
			{ record: 'engagement_requests', detail: `${engagement.kind}, asked and approved by ${this.ownerName}` },
			{ record: 'engagements', detail: `${engagement.kind}, due ${engagement.dueDate || 'not given'}. 1 Credit locked.` },
			engagement.doula === NO_DOULA
				? { record: 'attachments', detail: 'none written: No Doula yet' }
				: { record: 'attachments', detail: `granted: ${this.doulaLabel(engagement.doula)}, no fee, attached by ${this.ownerName}` }
		);
		this.go('engagement');
	}

	// A screen opened from the bar needs the records of the screens before it.
	go(screen: Screen) {
		const { sample } = this;
		if (screen !== 'signup' && !this.account) {
			this.account = { ...sample, practiceName: sample.practiceName || `${sample.firstName} ${sample.lastName}` };
		}
		if (['start', 'engagement', 'client'].includes(screen) && !this.client) {
			this.client = { ...sample.client };
		}
		if (screen === 'engagement' && !this.engagement) {
			this.engagement = { kind: 'birth', dueDate: sample.dueDate, doula: this.owner === 'solo' ? SELF : NO_DOULA, note: '' };
			this.balance = 2;
		}
		this.screen = screen;
	}

	startAgain() {
		this.account = undefined;
		this.client = undefined;
		this.engagement = undefined;
		this.balance = 3;
		this.presses = 0;
		this.log = [];
		this.fill = 0;
		this.note = '';
		this.run += 1;
		this.screen = 'signup';
	}
}

export const prototype = new Prototype();

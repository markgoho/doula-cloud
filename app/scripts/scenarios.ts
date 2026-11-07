// The named starting states `bun run seed:scenario <name>` builds (#2686):
// each one a state a walk once built by hand through the UI. Every write
// goes through the BFF or the Auth emulator (seedThroughBff.ts), so the
// audit rows are the product's own. One does not, and says why:
// returning-postpartum-client's backdated Engagement start.
//
// A scenario on its own solo Practice can run again on the same stack: its
// Owner's address carries a fresh suffix. A scenario on Rooted Birth
// Collective changes the seeded Practice, so it runs once per dev:full
// start.
import type { APIRequestContext } from '@playwright/test';
import { signIn } from '../e2e/auth';
import { API_URL, DEV_SERVER_ORIGIN } from '../e2e/ports';
import { backdateEngagementStart, MAILBOX_URL, simulatedDateAfter } from '../e2e/stack';
import { DEMO_ACCOUNTS, seedDemoPractice } from './demo-practice';
import {
	addClient,
	contractFor,
	dayAfter,
	DEMO_PASSWORD,
	demoAddress,
	instantAfter,
	invitePortal,
	portalSessionFor,
	send,
	signInWithPassword,
	signUpPractice,
	startAndSavePlan,
	type FoundedPractice,
	type NamedEngagement,
	type PracticeSession,
	type Session
} from './seedThroughBff';

/**
One account a walk signs in with.
*/
export interface ScenarioAccount {
	role: string;
	name: string;
	email: string;
	// Undefined for a Client, who signs in by a link from the mailbox, and
	// for someone who has only an Invitation so far.
	password?: string;
}

/**
What a scenario built: where the walk signs in and starts, and who with.
*/
export interface ScenarioResult {
	signInUrl: string;
	startUrl: string;
	practiceId: string;
	accounts: ScenarioAccount[];
	// The Owner's session, for a walk that drops it into a browser rather
	// than signing in.
	ownerCookie: string;
}

export interface Scenario {
	builds: string;
	flags?: Record<string, string>;
	run: (request: APIRequestContext, flags: ReadonlySet<string>) => Promise<ScenarioResult>;
}

const STAFF_SIGN_IN = `${DEV_SERVER_ORIGIN}/login`;
const PORTAL_SIGN_IN = `${DEV_SERVER_ORIGIN}/portal/login`;
const MAILBOX = `${MAILBOX_URL}/`;
const BIRTH_RATE_CENTS = 180_000;

function practiceUrl(practiceId: string): string {
	return `${DEV_SERVER_ORIGIN}/practices/${practiceId}`;
}

function cookieValue(session: Session): string {
	return session.Cookie.replace('__session=', '');
}

function suffix(): string {
	return Math.random().toString(36).slice(2, 8);
}

// Maya Okonkwo's solo Practice, signed up fresh: she is Owner, Admin and
// Doula at once.
async function signUpMaya(request: APIRequestContext): Promise<FoundedPractice & { account: ScenarioAccount }> {
	const email = `maya.okonkwo.${suffix()}@sim.doula.cloud`;
	const practice = await signUpPractice(request, {
		email,
		practiceName: 'Okonkwo Birth Support',
		givenName: 'Maya',
		familyName: 'Okonkwo',
		workState: 'NY'
	});
	return { ...practice, account: { role: 'Owner, Admin and Doula', name: 'Maya Okonkwo', email, password: DEMO_PASSWORD } };
}

async function setBirthRate(request: APIRequestContext, practice: PracticeSession): Promise<void> {
	await send(request, 'PUT', `/api/practices/${practice.practiceId}/rates/birth`, practice.owner, { amountCents: BIRTH_RATE_CENTS });
}

async function scheduleVisit(
	request: APIRequestContext,
	practice: PracticeSession,
	engagement: NamedEngagement,
	staffId: string,
	scheduledAt: string
): Promise<void> {
	await send(request, 'POST', `/api/practices/${practice.practiceId}/engagements/${engagement.engagementId}/visits`, practice.owner, {
		staffId,
		scheduledAt
	});
}

async function message(request: APIRequestContext, practice: PracticeSession, engagement: NamedEngagement, body: string): Promise<void> {
	await send(request, 'POST', `/api/practices/${practice.practiceId}/engagements/${engagement.engagementId}/messages`, practice.owner, {
		body
	});
}

async function recordOutcome(
	request: APIRequestContext,
	practiceId: string,
	engagementId: string,
	headers: Record<string, string>,
	birthOutcome: 'live_birth' | 'loss',
	pregnancyEndedOn: string
): Promise<void> {
	await send(request, 'PUT', `/api/practices/${practiceId}/engagements/${engagementId}/birth-outcome`, headers, {
		birthOutcome,
		pregnancyEndedOn
	});
}

function clientAccount(engagement: NamedEngagement): ScenarioAccount {
	return {
		role: 'Client',
		name: `${engagement.client.givenName} ${engagement.client.familyName}`,
		email: engagement.email
	};
}

// Three of the starter Birth Plan's questions answered, by the template's
// own labels (api/internal/staffauth/signup.go).
const THREE_BIRTH_PLAN_ANSWERS = {
	'Planned birth location': 'Hospital',
	'Preferences for atmosphere (music, lighting, etc.)': 'Lights low and my own playlist on quietly.',
	'Preferred pain relief': 'Open to options in the moment'
};

// solo-birth-doula.md's stage 7 and connect-onboarding.md: a solo Practice
// that answered the website question with its own site and has not
// connected Stripe, so Getting paid shows Connect Stripe. Nadia Haddad has
// signed her Contract, so an Invoice can be raised once Stripe is on.
async function connectReady(request: APIRequestContext, flags: ReadonlySet<string>): Promise<ScenarioResult> {
	const today = simulatedDateAfter(0);
	const maya = await signUpMaya(request);
	await setBirthRate(request, maya);
	// Stripe refuses a website on example.com as "Not a valid URL", so the
	// address is a plausible one (connect-onboarding.md).
	await send(request, 'PUT', `/api/practices/${maya.practiceId}/website`, maya.owner, {
		mode: 'own',
		ownUrl: 'https://okonkwobirthsupport.com',
		serviceDescription: '',
		cancellationPolicy: ''
	});
	const accounts = [maya.account];
	if (!flags.has('--no-client')) {
		const nadia = await addClient(
			request,
			maya,
			today,
			{ givenName: 'Nadia', familyName: 'Haddad', kind: 'birth', dueInDays: 100 },
			{ doulaStaffId: maya.staffId }
		);
		await contractFor(request, maya, today, nadia, await portalSessionFor(request, maya, nadia));
		accounts.push(clientAccount(nadia));
	}
	return {
		signInUrl: STAFF_SIGN_IN,
		startUrl: `${practiceUrl(maya.practiceId)}/settings/payments`,
		practiceId: maya.practiceId,
		accounts,
		ownerCookie: cookieValue(maya.owner)
	};
}

// evaluator-doula.md's walk 2 and solo-birth-doula.md's Practice B: a
// Practice a signup made minutes before, its email unverified, with
// nothing in it yet.
async function freshPractice(request: APIRequestContext): Promise<ScenarioResult> {
	const maya = await signUpMaya(request);
	return {
		signInUrl: STAFF_SIGN_IN,
		startUrl: practiceUrl(maya.practiceId),
		practiceId: maya.practiceId,
		accounts: [maya.account],
		ownerCookie: cookieValue(maya.owner)
	};
}

// first-time-client.md: Hannah Sorensen at Maya's Practice, birth work
// with Maya as her Doula, invited to the portal but not signed in yet, and
// a Birth Plan saved three times: three answers, one changed, then a save
// that changed nothing.
async function firstTimeClient(request: APIRequestContext): Promise<ScenarioResult> {
	const today = simulatedDateAfter(0);
	const maya = await signUpMaya(request);
	const hannah = await addClient(
		request,
		maya,
		today,
		{ givenName: 'Hannah', familyName: 'Sorensen', kind: 'birth', dueInDays: 128 },
		{ doulaStaffId: maya.staffId }
	);
	await invitePortal(request, maya, hannah);
	await startAndSavePlan(request, maya, hannah, 'birth_plan', maya.owner, THREE_BIRTH_PLAN_ANSWERS, [
		{ 'Preferred pain relief': 'Unmedicated' },
		{}
	]);
	return {
		signInUrl: PORTAL_SIGN_IN,
		startUrl: MAILBOX,
		practiceId: maya.practiceId,
		accounts: [clientAccount(hannah), maya.account],
		ownerCookie: cookieValue(maya.owner)
	};
}

// Nadia's two Visits already held and her next one, in days from today and
// the hour UTC.
const NADIA_VISITS: readonly (readonly [number, number])[] = [
	[-26, 15],
	[-12, 16],
	[11, 15]
];

// loss-client.md: Nadia Haddad at Maya's Practice, before her loss: a
// signed Contract, a Birth Plan, two past Visits and one booked, and a
// message from Maya. Rana Saleh has a loss recorded and no Birth Plan.
// --after-loss goes on to the state after walk 1: the loss recorded, a
// condolence message, and the Contract voided.
async function lossClient(request: APIRequestContext, flags: ReadonlySet<string>): Promise<ScenarioResult> {
	const today = simulatedDateAfter(0);
	const maya = await signUpMaya(request);
	await setBirthRate(request, maya);
	const nadia = await addClient(
		request,
		maya,
		today,
		{ givenName: 'Nadia', familyName: 'Haddad', kind: 'birth', dueInDays: 104 },
		{ doulaStaffId: maya.staffId }
	);
	await contractFor(request, maya, today, nadia, await portalSessionFor(request, maya, nadia));
	await startAndSavePlan(request, maya, nadia, 'birth_plan', maya.owner, THREE_BIRTH_PLAN_ANSWERS);
	for (const [days, hour] of NADIA_VISITS) {
		await scheduleVisit(request, maya, nadia, maya.staffId, instantAfter(today, days, hour));
	}
	await message(request, maya, nadia, 'Hi Nadia, I have your next visit in the calendar. Message me here any time.');

	const rana = await addClient(
		request,
		maya,
		today,
		{ givenName: 'Rana', familyName: 'Saleh', kind: 'birth', dueInDays: 90 },
		{ doulaStaffId: maya.staffId }
	);
	await scheduleVisit(request, maya, rana, maya.staffId, instantAfter(today, -9, 15));
	await recordOutcome(request, maya.practiceId, rana.engagementId, maya.owner, 'loss', dayAfter(today, -4));

	if (flags.has('--after-loss')) {
		await recordOutcome(request, maya.practiceId, nadia.engagementId, maya.owner, 'loss', dayAfter(today, -7));
		await message(request, maya, nadia, 'Nadia, I am so sorry. I am here whenever you want to talk.');
		await send(request, 'POST', `/api/practices/${maya.practiceId}/engagements/${nadia.engagementId}/contract/void`, maya.owner);
	}
	return {
		signInUrl: PORTAL_SIGN_IN,
		startUrl: PORTAL_SIGN_IN,
		practiceId: maya.practiceId,
		accounts: [clientAccount(nadia), maya.account],
		ownerCookie: cookieValue(maya.owner)
	};
}

const RENATA = demoAddress('Renata', 'Alvarez');
const PRIYA = demoAddress('Priya', 'Raman');
const LENA = demoAddress('Lena', 'Vasquez');

async function sessionFor(request: APIRequestContext, email: string): Promise<Session> {
	const idToken = await signInWithPassword(request, email);
	if (idToken === undefined) {
		throw new Error(`seed: ${email} has no account on this stack`);
	}
	return signIn(request, API_URL, idToken);
}

// Rooted Birth Collective, seeded now or found, with Renata's session.
async function rooted(request: APIRequestContext): Promise<PracticeSession> {
	const { practiceId } = await seedDemoPractice(request);
	return { practiceId, owner: await sessionFor(request, RENATA) };
}

function rootedAccount(email: string): ScenarioAccount {
	const account = DEMO_ACCOUNTS.find((candidate) => candidate.email === email);
	return { role: account?.role ?? 'Staff', name: account?.name ?? email, email, password: DEMO_PASSWORD };
}

interface ClientListItem {
	clientId: string;
	name: string;
	openEngagements?: { engagementId: string; kind: string }[];
}

// Every Client on the Practice's list, page by page.
async function listClients(request: APIRequestContext, practice: PracticeSession): Promise<ClientListItem[]> {
	const clients: ClientListItem[] = [];
	let cursor: string | undefined;
	for (;;) {
		const query = cursor === undefined ? '' : `?cursor=${encodeURIComponent(cursor)}`;
		const page = await send<{ items: ClientListItem[]; nextCursor?: string; hasMore: boolean }>(
			request,
			'GET',
			`/api/practices/${practice.practiceId}/clients${query}`,
			practice.owner
		);
		clients.push(...page.items);
		if (!page.hasMore || page.nextCursor === undefined) return clients;
		cursor = page.nextCursor;
	}
}

// A pending Staff Invitation at Rooted, which the walk accepts from the
// mailbox: employed-doula.md's and non-doula-admin.md's stage 1.
function invitedStaff(slug: string, name: string, roles: string[]): Scenario['run'] {
	return async (request) => {
		const renata = await rooted(request);
		const email = `${slug}.${suffix()}@sim.doula.cloud`;
		await send(request, 'POST', `/api/practices/${renata.practiceId}/staff/invitations`, renata.owner, {
			email,
			roles,
			employmentType: 'employee'
		});
		return {
			signInUrl: STAFF_SIGN_IN,
			startUrl: MAILBOX,
			practiceId: renata.practiceId,
			accounts: [{ role: `Invited: ${roles.join(', ')}, Employee`, name, email }, rootedAccount(RENATA)],
			ownerCookie: cookieValue(renata.owner)
		};
	};
}

// The fee Lena accepted on the seed's Offer, found the way Renata finds it:
// on the Engagement it belongs to.
async function lenasFee(request: APIRequestContext, renata: PracticeSession): Promise<{ engagementId: string; attachmentId: string }> {
	const clients = await listClients(request, renata);
	const engagementIds = clients.flatMap((client) => client.openEngagements ?? []).map(({ engagementId }) => engagementId);
	for (const engagementId of engagementIds) {
		const { items } = await send<{ items: { attachmentId: string; doulaName: string; paid?: unknown }[] }>(
			request,
			'GET',
			`/api/practices/${renata.practiceId}/engagements/${engagementId}/doula-fees`,
			renata.owner
		);
		const fee = items.find((item) => item.doulaName === 'Lena Vasquez' && item.paid === undefined);
		if (fee) return { engagementId, attachmentId: fee.attachmentId };
	}
	throw new Error('seed: Rooted holds no unpaid fee for Lena Vasquez');
}

// contractor-doula.md: Ridgeline Doulas, a second agency, has invited Lena
// Vasquez as a contractor Doula, and she accepts from the mailbox. At
// Rooted, Adaeze Nwosu's birth has no Doula yet and three Birth Plan
// answers, ready for an Offer, and Renata has recorded paying Lena's fee
// on the work Lena already holds there.
async function contractorSecondPractice(request: APIRequestContext): Promise<ScenarioResult> {
	const today = simulatedDateAfter(0);
	const renata = await rooted(request);

	const ridgelineEmail = `ridgeline.owner.${suffix()}@sim.doula.cloud`;
	const ridgeline = await signUpPractice(request, {
		email: ridgelineEmail,
		practiceName: 'Ridgeline Doulas',
		givenName: 'Beth',
		familyName: 'Alvarado',
		workState: 'NY'
	});
	await send(request, 'POST', `/api/practices/${ridgeline.practiceId}/staff/invitations`, ridgeline.owner, {
		email: LENA,
		roles: ['doula'],
		employmentType: 'contractor'
	});

	const adaeze = await addClient(request, renata, today, { givenName: 'Adaeze', familyName: 'Nwosu', kind: 'birth', dueInDays: 99 });
	await startAndSavePlan(request, renata, adaeze, 'birth_plan', renata.owner, THREE_BIRTH_PLAN_ANSWERS);

	const fee = await lenasFee(request, renata);
	await send(
		request,
		'POST',
		`/api/practices/${renata.practiceId}/engagements/${fee.engagementId}/doula-fees/${fee.attachmentId}/payments`,
		renata.owner,
		// Yesterday: `today` is the UTC day, which can be ahead of the
		// Practice's own, and a Fee Payment cannot be dated in the future.
		{ method: 'bank_transfer', paidOn: dayAfter(today, -1) }
	);
	return {
		signInUrl: STAFF_SIGN_IN,
		startUrl: MAILBOX,
		practiceId: renata.practiceId,
		accounts: [
			rootedAccount(LENA),
			rootedAccount(RENATA),
			{ role: 'Ridgeline Doulas Owner', name: 'Beth Alvarado', email: ridgelineEmail, password: DEMO_PASSWORD }
		],
		ownerCookie: cookieValue(renata.owner)
	};
}

// returning-postpartum-client.md: Camille Boyd's seeded birth Engagement
// ended -- a live birth recorded and care marked complete, with its booked
// Visit left on -- and started months ago. A second "Camille Boyd" with the
// same address, a different person, has birth work changed to postpartum,
// a Birth Plan started, a Draft Contract and a portal invitation.
//
// The one write here that is not the product's: the first Engagement's
// start, moved back by SQL (stack.ts's backdateEngagementStart). The clock
// cannot move, and without it both Engagements read as started today.
async function returningPostpartumClient(request: APIRequestContext): Promise<ScenarioResult> {
	const today = simulatedDateAfter(0);
	const renata = await rooted(request);
	const priya = await sessionFor(request, PRIYA);
	const { members } = await send<{ members: { staffId: string; email: string }[] }>(
		request,
		'GET',
		`/api/practices/${renata.practiceId}/staff`,
		renata.owner
	);
	const priyaStaffId = members.find((member) => member.email === PRIYA)?.staffId;
	if (priyaStaffId === undefined) throw new Error('seed: Rooted has no Priya Raman');

	const clients = await listClients(request, renata);
	const first = clients.find((client) => client.name === 'Camille Boyd')?.openEngagements?.[0];
	if (!first) throw new Error('seed: Rooted has no Camille Boyd with open work');
	await recordOutcome(request, renata.practiceId, first.engagementId, priya, 'live_birth', dayAfter(today, -8));
	await send(request, 'PATCH', `/api/practices/${renata.practiceId}/engagements/${first.engagementId}/status`, priya, {
		status: 'completed',
		endingReason: 'care_complete'
	});
	backdateEngagementStart(first.engagementId, 250);

	const second = await addClient(
		request,
		renata,
		today,
		{ givenName: 'Camille', familyName: 'Boyd', kind: 'birth', dueInDays: 74 },
		{ doulaStaffId: priyaStaffId, override: true }
	);
	await startAndSavePlan(request, renata, second, 'birth_plan', renata.owner, { 'Planned birth location': 'Home' });
	await send(request, 'PUT', `/api/practices/${renata.practiceId}/engagements/${second.engagementId}/kind`, renata.owner, {
		kind: 'postpartum'
	});
	const postpartum = { ...second, client: { ...second.client, kind: 'postpartum' as const } };
	await contractFor(request, renata, today, postpartum, undefined, { send: false });
	await invitePortal(request, renata, second);
	return {
		signInUrl: PORTAL_SIGN_IN,
		startUrl: PORTAL_SIGN_IN,
		practiceId: renata.practiceId,
		accounts: [clientAccount(second), rootedAccount(PRIYA), rootedAccount(RENATA)],
		ownerCookie: cookieValue(renata.owner)
	};
}

export const SCENARIOS: Record<string, Scenario> = {
	'connect-ready': {
		builds:
			'A fresh solo Practice (Maya Okonkwo, Owner, Admin and Doula) with its own website declared and Stripe not connected, so Getting paid shows Connect Stripe, and Nadia Haddad with a signed Contract.',
		flags: { '--no-client': 'leave Nadia out' },
		run: connectReady
	},
	'fresh-practice': {
		builds: 'A solo Practice a signup made just now, its email unverified, with nothing in it.',
		run: freshPractice
	},
	'first-time-client': {
		builds:
			"Hannah Sorensen at Maya's solo Practice: birth work with Maya as Doula, a portal invitation in the mailbox, and a Birth Plan saved three times.",
		run: firstTimeClient
	},
	'loss-client': {
		builds:
			"Nadia Haddad at Maya's solo Practice before her loss: signed Contract, Birth Plan, two past Visits and one booked, a message; and Rana Saleh with a loss recorded and no Birth Plan.",
		flags: { '--after-loss': "go on to Nadia's loss recorded, a condolence message and her Contract voided" },
		run: lossClient
	},
	'doula-invited': {
		builds: 'Rooted Birth Collective with a pending Invitation for a new employed Doula, to accept from the mailbox.',
		run: invitedStaff('priya.rewalk', 'Priya Ramanathan', ['doula'])
	},
	'admin-invited': {
		builds: 'Rooted Birth Collective with a pending Invitation for a new employed Admin who is not a Doula, to accept from the mailbox.',
		run: invitedStaff('dee.rewalk', 'Dee Whitlock', ['admin'])
	},
	'contractor-second-practice': {
		builds:
			"Rooted, plus Ridgeline Doulas inviting Lena Vasquez as a contractor; at Rooted, Adaeze Nwosu's birth with no Doula and three Birth Plan answers, and Lena's seeded fee recorded as paid.",
		run: contractorSecondPractice
	},
	'returning-postpartum-client': {
		builds:
			"Rooted, with Camille Boyd's birth care complete and started months ago, and a second Camille Boyd (same address) with postpartum work, a started Birth Plan, a Draft Contract and a portal invitation.",
		run: returningPostpartumClient
	}
};

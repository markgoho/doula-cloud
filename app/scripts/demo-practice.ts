// The local demo Practice (#1994): Rooted Birth Collective, seeded on every
// `bun run dev:full` start and by `bun run seed:demo`, with fixed sign-in
// accounts a person can use from the real `/login` form.
//
// It is the simulation's own World, not a second one: the roster is
// typicalPilot.ts's fifteen Rooted Staff (the World doc's cast), the
// anonymous book is world.ts's describeWorld tail, provisioned by
// provision.ts's provisionTail, and every Staff member joins through a real
// Invitation and acceptance. Each write goes through the BFF and the Auth
// emulator, the way the e2e helpers do, so the rows are what the product
// itself makes. One write cannot: an Invoice raised weeks ago, overdue or
// paid, because no endpoint raises an Invoice in the past (stack.ts's
// backdateInvoice says why).
//
// Two things the World leaves open, decided here for a demo:
//
// - Due dates are the World's, moved to start today. world.ts dates them
//   from the run's own day zero (2027-01-04); the demo moves that day to
//   the stack's current day, so the month-one births are inside an on-call
//   window now and the Schedule and On-call screens have something to show.
// - The book is two thirds of the World's tail, plus four named Clients for
//   Priya. Every approval spends a Credit, and the founding grant and the
//   signup bonus come to 48; the full tail would leave one.
//
// Local only. The BFF this talks to is the one e2e/stack.ts starts, against
// the Auth emulator and the sandbox mailbox; nothing here can reach the
// deployed app or Identity Platform, because every URL below is a loopback
// port from e2e/ports.ts.
import type { APIRequestContext } from '@playwright/test';
import { sessionCookieFrom, signIn } from '../e2e/auth';
import { drainOutbox, readMailbox } from '../e2e/outboxMail';
import { API_URL, DEV_SERVER_ORIGIN } from '../e2e/ports';
import { provisionTail, type DoulaSession, type ProvisionedClient } from '../e2e/simulation/provision';
import { describeWorld, type SeededClient } from '../e2e/simulation/world';
import { backdateInvoice, backdateInvoices, countPracticesNamed, MAILBOX_URL, readStaffInviteToken, seedFoundingGrant, seedStripeRail, simulatedDateAfter } from '../e2e/stack';
import { acceptStaffInvite } from '../e2e/staffSignup';
import { TYPICAL_PRACTICE_NAME, TYPICAL_STAFF, type TypicalStaff } from '../src/routes/typicalPilot';
import {
	addClient,
	contractFor,
	createIdentity,
	dayAfter,
	DEMO_PASSWORD,
	demoAddress,
	instantAfter,
	invitePortal,
	portalSessionFor,
	readOk,
	send,
	signInWithPassword,
	signUpPractice,
	startAndSavePlan,
	type NamedClient,
	type NamedEngagement,
	type Session
} from './seedThroughBff';

// The seed for describeWorld: the simulation's own, so the demo's tail is
// the same book a rehearsal run provisions.
const WORLD_SEED = 4_102_027;
// world.ts's day zero, which the demo moves to today.
const WORLD_DAY_ZERO = '2027-01-04';

function nameParts(member: TypicalStaff): { givenName: string; familyName: string } {
	const [givenName, familyName] = member.name.split(' ', 2);
	return { givenName, familyName };
}

// The slug world.ts and provisionTail key a Doula by: her given name.
function slugOf(member: TypicalStaff): string {
	return nameParts(member).givenName.toLowerCase();
}

function addressOf(member: TypicalStaff): string {
	const { givenName, familyName } = nameParts(member);
	return demoAddress(givenName, familyName);
}

const [OWNER, ADMIN, EMPLOYEE_DOULA, CONTRACTOR_DOULA] = TYPICAL_STAFF;
const INVITED = TYPICAL_STAFF.slice(1);

// Priya's book: the World gives her four Clients. Camille Boyd is Rooted's
// Client Persona and the demo's portal account.
const CAMILLE: NamedClient = { givenName: 'Camille', familyName: 'Boyd', kind: 'birth', dueInDays: 20 };
const PRIYA_CLIENTS: readonly NamedClient[] = [
	CAMILLE,
	{ givenName: 'Tessa', familyName: 'Moreau', kind: 'birth', dueInDays: 45 },
	{ givenName: 'Imani', familyName: 'Brooks', kind: 'postpartum' },
	{ givenName: 'Grace', familyName: 'Whitfield', kind: 'birth', dueInDays: 80 }
];

// One Client beyond the World, for the portal's after-a-loss state (#2003,
// #2007): Priya's too, so one Staff account walks both sides of it. Her
// pregnancy ended at about 30 weeks, ten weeks before she was due; her
// Engagement stays active, the state a Client is in when she next signs in.
// The World's own loss is Nadia Haddad's, at Maya's Practice, not Rooted.
const AFTER_A_LOSS: NamedClient = { givenName: 'Mira', familyName: 'Castellanos', kind: 'birth', dueInDays: 64 };
const PREGNANCY_ENDED_DAYS_AGO = 6;

const RATE_CENTS = { birth: 150_000, postpartum: 95_000 } as const;

/**
A fixed sign-in account, as the summary prints it.
*/
export interface DemoAccount {
	role: string;
	name: string;
	email: string;
}

export const DEMO_ACCOUNTS: readonly DemoAccount[] = [
	{ role: 'Owner', name: OWNER.name, email: addressOf(OWNER) },
	{ role: 'Admin (not a Doula)', name: ADMIN.name, email: addressOf(ADMIN) },
	{ role: 'Employee Doula', name: EMPLOYEE_DOULA.name, email: addressOf(EMPLOYEE_DOULA) },
	{ role: 'Contractor Doula', name: CONTRACTOR_DOULA.name, email: addressOf(CONTRACTOR_DOULA) }
];

export const DEMO_CLIENT_EMAIL = demoAddress(CAMILLE.givenName, CAMILLE.familyName);
export const AFTER_A_LOSS_CLIENT_EMAIL = demoAddress(AFTER_A_LOSS.givenName, AFTER_A_LOSS.familyName);

/**
Whole days from `from` to `to`, both YYYY-MM-DD.
*/
export function daysBetween(from: string, to: string): number {
	return Math.round((Date.parse(`${to}T12:00:00Z`) - Date.parse(`${from}T12:00:00Z`)) / 86_400_000);
}

/**
 * The demo's slice of the World's tail: two of every three Clients, in the
 * World's own order so each Doula keeps her share, with each birth due date
 * moved by the days from the World's day zero to `today`.
 */
export function demoTail(today: string): SeededClient[] {
	const shift = daysBetween(WORLD_DAY_ZERO, today);
	return describeWorld(WORLD_SEED)
		.tailClients.filter((_client, index) => index % 3 !== 2)
		.map((client) => ({ ...client, dueDate: client.dueDate === '' ? '' : dayAfter(client.dueDate, shift) }));
}

interface Roster {
	practiceId: string;
	owner: Session;
	ownerStaffId: string;
	// Every Staff member but the Owner, by slug (her given name).
	staff: Record<string, DoulaSession>;
}

// Renata signs up, the way a Practice starts.
async function signUpOwner(
	request: APIRequestContext
): Promise<{ practiceId: string; owner: Session; ownerStaffId: string }> {
	const { practiceId, staffId, owner } = await signUpPractice(request, {
		email: addressOf(OWNER),
		practiceName: TYPICAL_PRACTICE_NAME,
		...nameParts(OWNER),
		workState: OWNER.workState
	});
	return { practiceId, owner, ownerStaffId: staffId };
}

// One Staff member joins through the real Invitation: the Owner invites
// her, she makes her emulator account, and she accepts. The token is read
// out of the stack the way every e2e fixture reads it (stack.ts).
async function joinStaff(
	request: APIRequestContext,
	practiceId: string,
	owner: Session,
	member: TypicalStaff
): Promise<DoulaSession> {
	const email = addressOf(member);
	const { invitationId } = await send<{ invitationId: string }>(
		request,
		'POST',
		`/api/practices/${practiceId}/staff/invitations`,
		owner,
		{ email, roles: member.roles, employmentType: member.employmentType }
	);
	const inviteToken = await readStaffInviteToken(request, invitationId);
	const idToken = await createIdentity(request, email);
	const accepted = await acceptStaffInvite(request, idToken, {
		inviteToken,
		name: member.name,
		workState: member.workState
	});
	const { staffId } = JSON.parse(await readOk(accepted, `${member.name} accepting her invitation`));
	return { staffId, headers: sessionCookieFrom(accepted, 'accept-invite') };
}

async function buildRoster(request: APIRequestContext): Promise<Roster> {
	const { practiceId, owner, ownerStaffId } = await signUpOwner(request);
	const staff: Record<string, DoulaSession> = {};
	for (const member of INVITED) {
		staff[slugOf(member)] = await joinStaff(request, practiceId, owner, member);
	}
	// The pilot's founding grant (#439), sized from the roster just built:
	// three Credits a Staff member. ops gives it (#2482), so the seed
	// writes the operator's act directly.
	seedFoundingGrant(practiceId, 'Local demo seed');
	// A Contract takes its price from the rate for its kind (#967).
	await send(request, 'PUT', `/api/practices/${practiceId}/rates/birth`, owner, { amountCents: RATE_CENTS.birth });
	await send(request, 'PUT', `/api/practices/${practiceId}/rates/postpartum`, owner, { amountCents: RATE_CENTS.postpartum });
	return { practiceId, owner, ownerStaffId, staff };
}

// The tail across the three care phases. provisionTail leaves each
// Engagement in intake with its Doula attached; a Visit the Owner
// schedules for her moves it to active (ADR-0015's one automatic move);
// and a postpartum Engagement whose care is over is completed, which
// takes its outcome first (#940). One in four stays in intake.
async function spreadTailAcrossPhases(
	request: APIRequestContext,
	roster: Roster,
	today: string,
	tail: readonly ProvisionedClient[]
): Promise<void> {
	const { practiceId, owner, staff } = roster;
	for (const [index, client] of tail.entries()) {
		if (index % 4 === 0) continue;
		const engagementPath = `/api/practices/${practiceId}/engagements/${client.engagementId}`;
		const isFinished = isCareFinished(index, client);
		await send(request, 'POST', `${engagementPath}/visits`, owner, {
			staffId: staff[client.attachedDoulaSlug].staffId,
			scheduledAt: isFinished ? instantAfter(today, -40, 15) : instantAfter(today, 1 + (index % 12), 14 + (index % 5))
		});
		if (isFinished) {
			await send(request, 'PUT', `${engagementPath}/birth-outcome`, owner, {
				birthOutcome: 'live_birth',
				pregnancyEndedOn: dayAfter(today, -70)
			});
			await send(request, 'PATCH', `${engagementPath}/status`, owner, {
				status: 'completed',
				endingReason: 'care_complete'
			});
		} else if (client.kind === 'postpartum' && client.dueDate === '') {
			// Postpartum care under way began at a birth, so the Clients list's Due
			// column has a day to read ("Born ...") and not a dash.
			await send(request, 'PUT', `${engagementPath}/birth-outcome`, owner, {
				birthOutcome: 'live_birth',
				pregnancyEndedOn: dayAfter(today, -(10 + (index % 30)))
			});
		}
	}
}

// A named Client of Priya's: her record and Engagement, then Priya's own
// first Visit, which attaches her, as provisionTail does for the tail.
async function addPriyaClient(
	request: APIRequestContext,
	roster: Roster,
	today: string,
	client: NamedClient
): Promise<NamedEngagement> {
	const engagement = await addClient(request, roster, today, client);
	await send(
		request,
		'POST',
		`/api/practices/${roster.practiceId}/engagements/${engagement.engagementId}/visits`,
		roster.staff.priya.headers,
		{}
	);
	return engagement;
}

async function raiseInvoice(request: APIRequestContext, roster: Roster, engagement: NamedEngagement): Promise<string> {
	const { id } = await send<{ id: string }>(
		request,
		'POST',
		`/api/practices/${roster.practiceId}/engagements/${engagement.engagementId}/contract/invoices`,
		roster.owner,
		{ billingMode: 'by_hand' }
	);
	return id;
}

async function scheduleVisit(
	request: APIRequestContext,
	roster: Roster,
	engagement: NamedEngagement,
	staffId: string,
	scheduledAt: string
): Promise<void> {
	await send(request, 'POST', `/api/practices/${roster.practiceId}/engagements/${engagement.engagementId}/visits`, roster.owner, {
		staffId,
		scheduledAt
	});
}

// Priya's four. Camille Boyd is the demo's portal Client: her Contract is
// signed, an Invoice is open, Visits are logged and scheduled, and she and
// Priya have a thread. Tessa has paid, Imani is overdue, and Grace's
// Contract waits for her signature, still in intake.
async function seedPriyaBook(request: APIRequestContext, roster: Roster, today: string): Promise<NamedEngagement[]> {
	const book: NamedEngagement[] = [];
	for (const client of PRIYA_CLIENTS) {
		book.push(await addPriyaClient(request, roster, today, client));
	}
	const [camille, tessa, imani, grace] = book;
	const priya = roster.staff.priya;
	// Imani's baby came three weeks ago, which is why she is postpartum.
	await send(request, 'PUT', `/api/practices/${roster.practiceId}/engagements/${imani.engagementId}/birth-outcome`, priya.headers, {
		birthOutcome: 'live_birth',
		pregnancyEndedOn: dayAfter(today, -21)
	});
	const messagesPath = (engagement: NamedEngagement) =>
		`/api/practices/${roster.practiceId}/engagements/${engagement.engagementId}/messages`;

	const camillePortal = await portalSessionFor(request, roster, camille);
	await contractFor(request, roster, today, camille, camillePortal);
	await raiseInvoice(request, roster, camille);
	// Her Birth Plan, from the Practice's own template, so the portal's
	// Birth plan screen has one to show -- and both Plans saved once, by
	// two different people, so the Engagement page's "Last edited" rows
	// have a save to name (#2104). Her Birth Plan is in her own words, as
	// Priya took them down at the first visit; the Care Plan is the
	// doulas' own notes (#2137).
	await startAndSavePlan(request, roster, camille, 'birth_plan', priya.headers, {
		'Planned birth location': 'Hospital',
		'Preferences for atmosphere (music, lighting, etc.)':
			'Lights down low, my own playlist on quietly, and Marcus right beside me the whole time.',
		'Preferred pain relief': 'Open to options in the moment',
		'Anything else about pain relief': 'No epidural unless I ask twice.'
	});
	await startAndSavePlan(request, roster, camille, 'care_plan', roster.owner, {
		'Who will be present for support?': 'Marcus, her partner, all the way through. Her sister Dana until it is time to push.',
		'Special requests or concerns':
			'Iron was low at 28 weeks, so she is on supplements. Wants to labor at home for as long as it feels safe.'
	});
	const { visitId } = await send<{ visitId: string }>(
		request,
		'POST',
		`/api/practices/${roster.practiceId}/engagements/${camille.engagementId}/visits`,
		priya.headers,
		{}
	);
	await send(
		request,
		'PATCH',
		`/api/practices/${roster.practiceId}/engagements/${camille.engagementId}/visits/${visitId}/notes`,
		priya.headers,
		{ notes: 'First prenatal visit at home. Went through her birth preferences and the hospital bag list.' }
	);
	await scheduleVisit(request, roster, camille, priya.staffId, instantAfter(today, 3, 16));
	await send(request, 'POST', messagesPath(camille), priya.headers, {
		body: 'Hi Camille, it was lovely to meet you. I am on call for you from 37 weeks. Text or message me here any time.'
	});
	await send(request, 'POST', `/api/portal/engagements/${camille.engagementId}/messages`, camillePortal, {
		body: 'Thank you! Is it normal to feel more tired again this week?'
	});
	await send(request, 'POST', messagesPath(camille), priya.headers, {
		body: 'Very normal at this stage. Rest when you can, and we can talk it through at your next visit.'
	});
	// Her question with no reply yet, so the Messages section's
	// Unanswered row has something to say (#2104).
	await send(request, 'POST', `/api/portal/engagements/${camille.engagementId}/messages`, camillePortal, {
		body: 'Thank you. Could we move Thursday’s visit to the afternoon?'
	});

	await contractFor(request, roster, today, tessa, await portalSessionFor(request, roster, tessa));
	const paid = await raiseInvoice(request, roster, tessa);
	await send(request, 'POST', `/api/practices/${roster.practiceId}/invoices/${paid}/payments`, roster.owner, {
		method: 'check',
		note: 'Check 1042',
		paidOn: dayAfter(today, -2)
	});
	backdateInvoice(paid, 1);
	await scheduleVisit(request, roster, tessa, priya.staffId, instantAfter(today, 7, 15));
	await send(request, 'POST', messagesPath(tessa), priya.headers, {
		body: 'Your check came in, thank you. See you next week.'
	});

	await contractFor(request, roster, today, imani, await portalSessionFor(request, roster, imani));
	backdateInvoice(await raiseInvoice(request, roster, imani), 12);
	await scheduleVisit(request, roster, imani, priya.staffId, instantAfter(today, 2, 23));

	await invitePortal(request, roster, grace);
	await contractFor(request, roster, today, grace, undefined);
	return book;
}

// Mira Castellanos: a signed Contract and a logged Visit, then the loss,
// recorded the way a Staff member records it (#294) -- the portal reads
// the outcome to show its after-a-loss state.
async function seedAfterALoss(request: APIRequestContext, roster: Roster, today: string): Promise<void> {
	const mira = await addPriyaClient(request, roster, today, AFTER_A_LOSS);
	await contractFor(request, roster, today, mira, await portalSessionFor(request, roster, mira));
	// Care was under way before the loss: a Visit on a day, which moves the
	// Engagement from intake to active (ADR-0015), so the list reads "Active
	// · Contract signed" beside the loss and not "Intake" with a signed Contract.
	await scheduleVisit(request, roster, mira, roster.staff.priya.staffId, instantAfter(today, -(PREGNANCY_ENDED_DAYS_AGO + 14), 15));
	await send(
		request,
		'PUT',
		`/api/practices/${roster.practiceId}/engagements/${mira.engagementId}/birth-outcome`,
		roster.staff.priya.headers,
		{ birthOutcome: 'loss', pregnancyEndedOn: dayAfter(today, -PREGNANCY_ENDED_DAYS_AGO) }
	);
}

function sendOffer(
	request: APIRequestContext,
	roster: Roster,
	client: ProvisionedClient,
	staffId: string,
	amountCents: number
): Promise<{ offerId: string }> {
	return send<{ offerId: string }>(
		request,
		'POST',
		`/api/practices/${roster.practiceId}/engagements/${client.engagementId}/offers`,
		roster.owner,
		{
			staffId,
			amountCents,
			terms: 'Backup for the birth, on call from 38 weeks, one prenatal visit.',
			clientFirstInitial: client.givenName.slice(0, 1),
			clientArea: 'South Wedge',
			dueDate: client.dueDate
		}
	);
}

// The contractor's door: an Offer on a tail birth, which Lena accepts and
// then visits, and a second Offer to Fern that she has not answered yet.
async function seedOffers(
	request: APIRequestContext,
	roster: Roster,
	today: string,
	tail: readonly ProvisionedClient[]
): Promise<void> {
	const births = tail.filter((client) => client.kind === 'birth');
	const offer = (client: ProvisionedClient, staffId: string, amountCents: number) =>
		sendOffer(request, roster, client, staffId, amountCents);
	const lena = roster.staff.lena;
	const { offerId } = await offer(births[0], lena.staffId, 45_000);
	await send(request, 'POST', `/api/practices/${roster.practiceId}/offers/${offerId}/accept`, lena.headers);
	await send(request, 'POST', `/api/practices/${roster.practiceId}/engagements/${births[0].engagementId}/visits`, lena.headers, {
		scheduledAt: instantAfter(today, 5, 17)
	});
	await offer(births[1], roster.staff.fern.staffId, 50_000);
}

type PaymentMethod = 'check' | 'bank_transfer' | 'cash' | 'other';

interface BookInvoice {
	// A share of the Contract's price, to the nearest $5; omitted, what the
	// Contract has left to bill.
	share?: number;
	// Over 30, moved back by backdateInvoices; omitted, raised today.
	raisedDaysAgo?: number;
	// Omitted, the Practice's 30-day terms from the day it was raised.
	dueInDays?: number;
	// Moved onto Stripe's rail by seedStripeRail: a Stripe-billed row, or one
	// whose payment is on its way. An open Invoice only.
	rail?: 'stripe' | 'on-its-way';
	paid?: { method: PaymentMethod; note?: string; daysAgo: number };
	after?: 'write-off' | 'void' | { reverse: string } | { refundCents: number; method: PaymentMethod; note: string };
}

const BOOK_INVOICES: readonly (readonly BookInvoice[])[] = [
	[
		{ share: 1 / 2, raisedDaysAgo: 95, paid: { method: 'check', note: 'Check 2217', daysAgo: 90 } },
		{ raisedDaysAgo: 75 }
	],
	[
		{
			raisedDaysAgo: 42,
			paid: { method: 'bank_transfer', daysAgo: 37 },
			after: { refundCents: 30_000, method: 'check', note: 'Two prenatal visits not used' }
		}
	],
	[{ raisedDaysAgo: 38, paid: { method: 'check', note: 'Check 5530', daysAgo: 33 }, after: { reverse: 'The check was returned unpaid by the bank' } }],
	[{ raisedDaysAgo: 65, after: 'write-off' }],
	[{ share: 0.4, after: 'void' }, { dueInDays: 21 }],
	[{ share: 1 / 3, raisedDaysAgo: 35, dueInDays: 10 }],
	[{ raisedDaysAgo: 33 }],
	[{ share: 1 / 3, raisedDaysAgo: 50, paid: { method: 'cash', daysAgo: 47 } }, {}],
	[{ raisedDaysAgo: 40, paid: { method: 'bank_transfer', daysAgo: 12 } }],
	[{}],
	[{ share: 1 / 3, raisedDaysAgo: 45, paid: { method: 'check', note: 'Check 318', daysAgo: 41 } }, { dueInDays: 60 }],
	[{ raisedDaysAgo: 57, rail: 'on-its-way' }],
	[{ share: 1 / 3, dueInDays: 7, rail: 'stripe' }],
	[{ share: 1 / 3, raisedDaysAgo: 130, paid: { method: 'cash', daysAgo: 126 } }, { raisedDaysAgo: 92 }],
	[{ raisedDaysAgo: 44, paid: { method: 'other', note: 'Venmo', daysAgo: 40 } }],
	[
		{ share: 1 / 3, raisedDaysAgo: 120, paid: { method: 'bank_transfer', daysAgo: 118 } },
		{ share: 1 / 3, raisedDaysAgo: 60, paid: { method: 'check', note: 'Check 7781', daysAgo: 55 } },
		{ raisedDaysAgo: 35, dueInDays: 25 }
	],
	[{ raisedDaysAgo: 34, paid: { method: 'bank_transfer', daysAgo: 31 } }],
	[{ share: 1 / 2, raisedDaysAgo: 36, paid: { method: 'cash', daysAgo: 36 } }, { dueInDays: 18 }],
	[{ raisedDaysAgo: 49, rail: 'stripe' }],
	[{}],
	[{ raisedDaysAgo: 31, paid: { method: 'check', note: 'Check 1187', daysAgo: 2 } }],
	[{ raisedDaysAgo: 90, paid: { method: 'bank_transfer', daysAgo: 85 } }],
	[{ share: 1 / 3, raisedDaysAgo: 33, paid: { method: 'bank_transfer', daysAgo: 30 } }],
	[{ share: 1 / 6, raisedDaysAgo: 32, paid: { method: 'cash', daysAgo: 32 } }]
];

function isCareFinished(index: number, client: ProvisionedClient): boolean {
	return index % 4 === 3 && client.kind === 'postpartum';
}

// An agency's Invoice book across the tail (#2746): mostly paid, some owed
// and not yet due, several overdue by different days, deposits and
// balances on one Contract, and a refund, a bounced check, a write-off and
// a void, each young enough to be on the list's first page. Not on the two
// Engagements the Offers are on, nor on finished care. Raised after
// Priya's three, so INV-0001 to INV-0003 stay hers. A
// tail Client has no address, and accepting a portal invitation needs one.
async function seedInvoiceBook(
	request: APIRequestContext,
	roster: Roster,
	today: string,
	tail: readonly ProvisionedClient[]
): Promise<void> {
	const { practiceId, owner } = roster;
	const offered = new Set(
		tail
			.filter((client) => client.kind === 'birth')
			.slice(0, 2)
			.map((client) => client.engagementId)
	);
	const billed = tail.filter((client, index) => !offered.has(client.engagementId) && !isCareFinished(index, client));
	const moves: { invoiceId: string; days: number }[] = [];
	const onStripe: { invoiceId: string; paymentOnItsWay: boolean }[] = [];
	for (const [index, client] of billed.entries()) {
		const invoices = BOOK_INVOICES[index];
		if (invoices === undefined) break;
		const { givenName, familyName, kind } = client;
		const email = demoAddress(givenName, familyName);
		await send(request, 'PUT', `/api/practices/${practiceId}/clients/${client.clientId}`, owner, {
			givenName,
			familyName,
			email,
			override: true
		});
		const engagement: NamedEngagement = {
			client: { givenName, familyName, kind },
			clientId: client.clientId,
			engagementId: client.engagementId,
			email
		};
		await contractFor(request, roster, today, engagement, await portalSessionFor(request, roster, engagement));
		for (const invoice of invoices) {
			const raisedDaysAgo = invoice.raisedDaysAgo ?? 0;
			const { id } = await send<{ id: string }>(
				request,
				'POST',
				`/api/practices/${practiceId}/engagements/${client.engagementId}/contract/invoices`,
				owner,
				{
					billingMode: 'by_hand',
					amountCents: invoice.share === undefined ? undefined : Math.round((RATE_CENTS[kind] * invoice.share) / 500) * 500,
					dueOn: invoice.dueInDays === undefined ? undefined : dayAfter(today, invoice.dueInDays + raisedDaysAgo)
				}
			);
			if (raisedDaysAgo > 0) moves.push({ invoiceId: id, days: raisedDaysAgo - 30 });
			if (invoice.rail !== undefined) onStripe.push({ invoiceId: id, paymentOnItsWay: invoice.rail === 'on-its-way' });
			const invoicePath = `/api/practices/${practiceId}/invoices/${id}`;
			const payment = invoice.paid
				? await send<{ id: string }>(request, 'POST', `${invoicePath}/payments`, owner, {
						method: invoice.paid.method,
						note: invoice.paid.note,
						paidOn: dayAfter(today, -invoice.paid.daysAgo)
					})
				: undefined;
			const after = invoice.after;
			if (after === 'write-off' || after === 'void') {
				await send(request, 'POST', `${invoicePath}/${after}`, owner);
			} else if (after !== undefined && payment !== undefined) {
				const paymentPath = `${invoicePath}/payments/${payment.id}`;
				if ('reverse' in after) {
					await send(request, 'POST', `${paymentPath}/reverse`, owner, { reason: after.reverse });
				} else {
					await send(request, 'POST', `${paymentPath}/refund`, owner, {
						amountCents: after.refundCents,
						method: after.method,
						note: after.note
					});
				}
			}
		}
	}
	backdateInvoices(moves);
	seedStripeRail(onStripe);
}

// RFC 3339 for `days` after `day` at hourUtc:minute; an hour past 23 runs
// into the next UTC day, which is how an evening Eastern time is written.
function instantAt(day: string, days: number, hourUtc: number, minute: number): string {
	const start = Date.parse(`${dayAfter(day, days)}T00:00:00Z`);
	return new Date(start + (hourUtc * 60 + minute) * 60_000).toISOString().replace('.000Z', 'Z');
}

// The next :00 or :30 at least `minutes` from now.
function laterToday(minutes: number): string {
	const slot = 30 * 60_000;
	const at = Math.ceil((Date.now() + minutes * 60_000) / slot) * slot;
	return new Date(at).toISOString().replace('.000Z', 'Z');
}

// [Doula slug, days from today, hour UTC, minute]. Eastern is UTC-5 or -4:
// 12:30 is before 9am there, 25:00 is 8 or 9pm.
const SCHEDULE_BOOK: readonly (readonly [string, number, number, number])[] = [
	['joss', 1, 14, 0],
	['marisol', 4, 12, 30],
	['rowan', 9, 14, 0],
	['rowan', 9, 17, 30],
	['delia', 9, 15, 0],
	['kimiko', 9, 13, 30],
	['joss', 9, 16, 0],
	['aditi', 13, 14, 0],
	['marisol', 15, 18, 30],
	['bethany', 17, 14, 30],
	['joss', 18, 25, 0],
	['rowan', 21, 15, 30],
	['bethany', 26, 12, 30],
	['delia', 27, 14, 30],
	['charlene', 29, 15, 0],
	['joss', -2, 15, 0],
	['marisol', -3, 14, 30],
	['bethany', -5, 16, 0],
	['aditi', -6, 13, 30],
	['delia', -1, 15, 30]
];

// The Practice-wide Schedule's book (#2747): a month of Visits across the
// Doulas, two later today, five in the last week, and two booked Visits whose
// Doula was then taken off their Engagement (#2596), one on the first page of
// the Schedule and one behind "Load more". Each Visit is held by the
// Engagement's own Doula, so it attaches no one; Renata is a backup on one
// Engagement. Only on care already active, so no Client's status moves (a
// Visit on intake care moves it to active, ADR-0015).
async function seedScheduleBook(
	request: APIRequestContext,
	roster: Roster,
	today: string,
	tail: readonly ProvisionedClient[]
): Promise<void> {
	const active = tail.filter((client, index) => index % 4 !== 0 && !isCareFinished(index, client));
	const postpartum = active.filter((client) => client.kind === 'postpartum');
	const [takenOff, ...rest] = postpartum;
	const lateTakenOff = rest.find((client) => client.attachedDoulaSlug !== 'kimiko')!;
	const pools = new Map<string, ProvisionedClient[]>();
	for (const client of active) {
		if (client === takenOff || client === lateTakenOff) continue;
		pools.set(client.attachedDoulaSlug, [...(pools.get(client.attachedDoulaSlug) ?? []), client]);
	}
	const turns = new Map<string, number>();
	const clientOf = (slug: string) => {
		const own = pools.get(slug)!;
		const turn = turns.get(slug) ?? 0;
		turns.set(slug, turn + 1);
		return own[turn % own.length];
	};
	const engagementPath = (engagementId: string) => `/api/practices/${roster.practiceId}/engagements/${engagementId}`;
	const staffIds: Record<string, string> = {
		renata: roster.ownerStaffId,
		...Object.fromEntries(Object.entries(roster.staff).map(([slug, member]) => [slug, member.staffId]))
	};
	const visit = (client: ProvisionedClient, slug: string, scheduledAt: string) =>
		send(request, 'POST', `${engagementPath(client.engagementId)}/visits`, roster.owner, {
			staffId: staffIds[slug],
			scheduledAt
		});
	const entries: (readonly [string, string])[] = [
		...SCHEDULE_BOOK.map(([slug, days, hour, minute]) => [slug, instantAt(today, days, hour, minute)] as const),
		['aditi', laterToday(45)],
		['kimiko', laterToday(100)]
	];
	for (const [slug, scheduledAt] of entries) {
		await visit(clientOf(slug), slug, scheduledAt);
	}
	await visit(pools.get('delia')![0], 'renata', instantAt(today, 24, 16, 0));
	await visit(takenOff, 'renata', instantAt(today, 16, 19, 30));
	await send(
		request,
		'DELETE',
		`${engagementPath(takenOff.engagementId)}/attachments/${staffIds[takenOff.attachedDoulaSlug]}`,
		roster.owner
	);
	await visit(lateTakenOff, 'kimiko', instantAt(today, 20, 16, 0));
	await send(
		request,
		'DELETE',
		`${engagementPath(lateTakenOff.engagementId)}/attachments/${staffIds.kimiko}`,
		roster.owner
	);
}

const PRACTICE_ZONE = 'America/New_York';

function inPracticeZone(at: Date): { day: string; hour: number } {
	const parts = Object.fromEntries(
		new Intl.DateTimeFormat('en-CA', {
			timeZone: PRACTICE_ZONE,
			year: 'numeric',
			month: '2-digit',
			day: '2-digit',
			hour: '2-digit',
			hourCycle: 'h23'
		})
			.formatToParts(at)
			.map((part) => [part.type, part.value])
	);
	return { day: `${parts.year}-${parts.month}-${parts.day}`, hour: Number(parts.hour) };
}

// The instant `hour` o'clock on `day` in the Practice's zone.
function atHourInPracticeZone(day: string, hour: number): Date {
	for (const offsetHours of [4, 5]) {
		const at = new Date(Date.parse(`${day}T00:00:00Z`) + (hour + offsetHours) * 3_600_000);
		const seen = inPracticeZone(at);
		if (seen.day === day && seen.hour === hour) return at;
	}
	throw new Error(`demo seed: no instant at ${hour}:00 on ${day} in ${PRACTICE_ZONE}`);
}

// 6pm tonight to 10am tomorrow in the Practice's zone, started at now when
// 6pm has passed; before 10am it is the night already under way.
function tonight(now: Date): { startsAt: string; endsAt: string } {
	const { day, hour } = inPracticeZone(now);
	const isEarly = hour < 10;
	const startsAt = isEarly ? now : new Date(Math.max(atHourInPracticeZone(day, 18).getTime(), now.getTime()));
	const endsAt = atHourInPracticeZone(isEarly ? day : dayAfter(day, 1), 10);
	return { startsAt: startsAt.toISOString(), endsAt: endsAt.toISOString() };
}

// What the On call screen shows beyond a plain Covered row, each on its own
// birth. Morgan Grier has Marisol as primary and Lena as backup by
// Narrowing, and a gap Lena covers. Casey Estrada's Doula, Rowan, is
// narrowed to leave the last week with nobody. Priya is away tonight with
// nobody covering tonight on Camille's birth, where Bethany takes over from
// the due date. Imogen has a Doula and no due date, so no window yet. Tamsin
// Reyes is due in 25 days with no Doula. Quinn Estrada has her own start rule
// and Tessa Moreau stays plainly Covered. Dee has changed the Practice's rule
// twice, so its Activity holds two entries.
async function seedOnCall(
	request: APIRequestContext,
	roster: Roster,
	today: string,
	tail: readonly ProvisionedClient[],
	camille: NamedEngagement
): Promise<void> {
	const { practiceId, owner, staff } = roster;
	const birth = (givenName: string, familyName: string) =>
		tail.find((client) => client.kind === 'birth' && client.givenName === givenName && client.familyName === familyName)!;
	const engagementPath = (engagementId: string) => `/api/practices/${practiceId}/engagements/${engagementId}`;
	const narrow = (engagementId: string, staffId: string, from: string | undefined, to: string | undefined) =>
		send(request, 'PUT', `${engagementPath(engagementId)}/attachments/${staffId}/on-call`, owner, { from, to });
	const gap = (engagementId: string, body: object) =>
		send(request, 'POST', `${engagementPath(engagementId)}/coverage-gaps`, owner, body);

	const practiceRule = { startRule: 'gestational_week', startWeek: 37, graceDays: 14 };
	for (const startWeek of [38, 37]) {
		await send(request, 'PUT', `/api/practices/${practiceId}/on-call-settings`, staff.dee.headers, {
			...practiceRule,
			startWeek
		});
	}

	const morgan = birth('Morgan', 'Grier');
	const lena = staff.lena;
	const { offerId } = await sendOffer(request, roster, morgan, lena.staffId, 45_000);
	await send(request, 'POST', `/api/practices/${practiceId}/offers/${offerId}/accept`, lena.headers);
	await narrow(morgan.engagementId, staff.marisol.staffId, undefined, dayAfter(today, 10));
	await narrow(morgan.engagementId, lena.staffId, dayAfter(today, 11), undefined);
	await gap(morgan.engagementId, {
		staffId: staff.marisol.staffId,
		startsAt: instantAt(today, 3, 22, 0),
		endsAt: instantAt(today, 4, 14, 0),
		reason: 'Her daughter’s recital in Buffalo',
		coveringStaffId: lena.staffId
	});

	await narrow(birth('Casey', 'Estrada').engagementId, staff.rowan.staffId, undefined, dayAfter(today, 8));

	await send(request, 'PUT', `${engagementPath(birth('Quinn', 'Estrada').engagementId)}/on-call-rule`, owner, {
		startRule: 'attachment_granted'
	});

	await send(request, 'PUT', `${engagementPath(camille.engagementId)}/attachments/${staff.bethany.staffId}`, owner);
	await narrow(camille.engagementId, staff.priya.staffId, undefined, dayAfter(today, 19));
	await narrow(camille.engagementId, staff.bethany.staffId, dayAfter(today, 20), undefined);

	await gap(camille.engagementId, {
		staffId: staff.priya.staffId,
		...tonight(new Date()),
		reason: 'At a wedding four hours away'
	});

	await addClient(
		request,
		roster,
		today,
		{ givenName: 'Imogen', familyName: 'Okafor', kind: 'birth' },
		{ doulaStaffId: staff.delia.staffId }
	);

	await addClient(request, roster, today, { givenName: 'Tamsin', familyName: 'Reyes', kind: 'birth', dueInDays: 25 });
}

export interface DemoPractice {
	practiceId: string;
	// false when this call found the demo Practice already there and wrote
	// nothing to it.
	seeded: boolean;
}

// The demo Practice this stack already holds, or undefined on a fresh
// stack. Renata's password sign-in is the probe: her account is there only
// once the seed has run. A roster short of fifteen means a seed that
// stopped part way, which this refuses to build on.
//
// No account for her, but the Practice in the database, is a stack killed
// without its teardown (#2010): the Auth emulator keeps nothing between
// starts, the database volume does. Seeding on top would sign Renata up
// again and make a second Rooted Birth Collective, so this refuses.
// dev:full never meets it, because it starts from a clean stack; a
// `seed:demo` against such a stack does.
async function findDemoPractice(request: APIRequestContext): Promise<DemoPractice | undefined> {
	const idToken = await signInWithPassword(request, addressOf(OWNER));
	if (idToken === undefined) {
		const stale = countPracticesNamed(TYPICAL_PRACTICE_NAME);
		if (stale > 0) {
			throw new Error(
				`demo seed: the database holds ${TYPICAL_PRACTICE_NAME}, but the Auth emulator has no account for ${addressOf(OWNER)}. The stack was stopped without its teardown. Stop dev:full and start it again: it starts from a clean stack.`
			);
		}
		return undefined;
	}
	const owner = await signIn(request, API_URL, idToken);
	const { memberships } = await send<{ memberships: { practiceId: string; practiceName: string }[] }>(
		request,
		'GET',
		'/api/staff/session',
		owner
	);
	const practice = memberships.find((membership) => membership.practiceName === TYPICAL_PRACTICE_NAME);
	const { members } = practice
		? await send<{ members: unknown[] }>(request, 'GET', `/api/practices/${practice.practiceId}/staff`, owner)
		: { members: [] };
	if (!practice || members.length !== TYPICAL_STAFF.length) {
		throw new Error(
			`demo seed: ${addressOf(OWNER)} exists, but ${TYPICAL_PRACTICE_NAME} is not whole (${members.length} of ${TYPICAL_STAFF.length} Staff). An earlier seed stopped part way; stop dev:full and start it again for a clean stack.`
		);
	}
	return { practiceId: practice.practiceId, seeded: false };
}

/**
 * Seeds the demo Practice, or finds it already there and changes nothing.
 * Safe to call against a stack in any state but a half-seeded one, which
 * it names and refuses.
 */
export async function seedDemoPractice(request: APIRequestContext): Promise<DemoPractice> {
	const existing = await findDemoPractice(request);
	if (existing) return existing;

	const today = simulatedDateAfter(0);
	const roster = await buildRoster(request);
	const tail = await provisionTail(request, roster.practiceId, roster.owner, roster.staff, demoTail(today));
	await spreadTailAcrossPhases(request, roster, today, tail);
	const [camille] = await seedPriyaBook(request, roster, today);
	await seedAfterALoss(request, roster, today);
	await seedOffers(request, roster, today, tail);
	await seedInvoiceBook(request, roster, today, tail);
	await seedScheduleBook(request, roster, today, tail);
	await seedOnCall(request, roster, today, tail, camille);
	return { practiceId: roster.practiceId, seeded: true };
}

const MAGIC_LINK_SUBJECT = 'Your DoulaCloud sign-in link';
const MAGIC_LINK = /https?:\/\/\S+\/portal\/sign-in\?token=\S+/;
const MAIL_TIMEOUT_MS = 10_000;

/**
 * A fresh portal sign-in link for one demo Client, the way the portal signs
 * a Client in locally: request it, drain the outbox as Cloud Scheduler
 * would, and read it out of the sandbox mailbox. Only mail newer than the
 * request counts, so a link an earlier run printed is never handed back.
 * It lasts 15 minutes and works once (clientauth.MagicLinkLifetime).
 * dev:full's own drain may send it first; the mailbox read finds it either
 * way.
 */
export async function mintPortalLink(request: APIRequestContext, email: string): Promise<string> {
	const before = await readMailbox(request, email);
	const newestBefore = Math.max(0, ...before.map((message) => message.seq));
	await send(request, 'POST', '/api/portal/magic-link/request', {}, { email });
	const deadline = Date.now() + MAIL_TIMEOUT_MS;
	for (;;) {
		await drainOutbox(request, 'process-portal-magic-link-outbox');
		const inbox = await readMailbox(request, email);
		const mail = inbox.find((message) => message.seq > newestBefore && message.subject === MAGIC_LINK_SUBJECT);
		const link = mail?.text.match(MAGIC_LINK)?.[0];
		if (link) return link;
		if (Date.now() > deadline) {
			throw new Error(`demo seed: no sign-in link reached ${email} within ${MAIL_TIMEOUT_MS}ms`);
		}
		await new Promise((resolve) => setTimeout(resolve, 250));
	}
}

/**
The demo's portal Clients, each with a fresh sign-in link.
*/
export interface PortalLinks {
	camille: string;
	afterALoss: string;
}

export async function mintPortalLinks(request: APIRequestContext): Promise<PortalLinks> {
	return {
		camille: await mintPortalLink(request, DEMO_CLIENT_EMAIL),
		afterALoss: await mintPortalLink(request, AFTER_A_LOSS_CLIENT_EMAIL)
	};
}

/**
What a person needs to sign in, as the lines dev:full prints.
*/
export function demoSummary(practice: DemoPractice, links: PortalLinks): string {
	const practiceUrl = `${DEV_SERVER_ORIGIN}/practices/${practice.practiceId}`;
	const width = Math.max(...DEMO_ACCOUNTS.map((account) => account.role.length));
	return [
		'',
		`${TYPICAL_PRACTICE_NAME}, the local demo Practice (${practice.seeded ? 'seeded now' : 'already seeded, nothing changed'})`,
		'',
		`  Staff sign in at ${DEV_SERVER_ORIGIN}/login, password ${DEMO_PASSWORD}`,
		...DEMO_ACCOUNTS.map((account) => `    ${account.role.padEnd(width)}  ${account.email}  (${account.name})`),
		'    Every other Rooted Staff member signs in the same way, at given.family@sim.doula.cloud.',
		`  The Practice: ${practiceUrl}`,
		'',
		'  Clients have no password. Each sign-in link is good for 15 minutes and one use.',
		`    ${DEMO_CLIENT_EMAIL} (Camille Boyd): ${links.camille}`,
		`    ${AFTER_A_LOSS_CLIENT_EMAIL} (Mira Castellanos, after a loss): ${links.afterALoss}`,
		`    For a new link, ask for one at ${DEV_SERVER_ORIGIN}/portal/login, or run: bun run --cwd app seed:demo`,
		'',
		`  Local mail, sign-in links and Invitations among it: ${MAILBOX_URL}/`,
		''
	].join('\n');
}

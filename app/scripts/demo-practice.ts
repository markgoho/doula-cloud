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
// itself makes. One write cannot: an overdue Invoice, because no endpoint
// raises an Invoice in the past (stack.ts's backdateInvoice says why).
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
import type { APIRequestContext, APIResponse } from '@playwright/test';
import { sessionCookieFrom, signIn } from '../e2e/auth';
import { drainOutbox, readMailbox } from '../e2e/outboxMail';
import { API_URL, DEV_SERVER_ORIGIN, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from '../e2e/ports';
import { retryPastRateLimit } from '../e2e/rateLimit';
import { provisionTail, type DoulaSession, type ProvisionedClient } from '../e2e/simulation/provision';
import { describeWorld, type SeededClient } from '../e2e/simulation/world';
import { backdateInvoice, countPracticesNamed, MAILBOX_URL, readStaffInviteToken, simulatedDateAfter, WORKER_SECRET } from '../e2e/stack';
import { acceptStaffInvite } from '../e2e/staffSignup';
import { TYPICAL_PRACTICE_NAME, TYPICAL_STAFF, type TypicalStaff } from '../src/routes/typicalPilot';

/**
 * The one password every demo account signs in with. Dev-only and fake on
 * purpose: it exists in the local Auth emulator and nowhere else, and the
 * emulator is wiped each time the stack stops. Fifteen characters or more,
 * the Staff rule (#1538).
 */
export const DEMO_PASSWORD = 'demo-password-local-only';

// The domain the simulation already gives a simulated person's address
// (docs/simulation/environment.md), so a log says at a glance that the
// address is nobody's. Mail to it goes to the sandbox mailbox only.
const DEMO_DOMAIN = 'sim.doula.cloud';

// The seed for describeWorld: the simulation's own, so the demo's tail is
// the same book a rehearsal run provisions.
const WORLD_SEED = 4_102_027;
// world.ts's day zero, which the demo moves to today.
const WORLD_DAY_ZERO = '2027-01-04';

const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;

export function demoAddress(givenName: string, familyName: string): string {
	return `${givenName}.${familyName}@${DEMO_DOMAIN}`.toLowerCase();
}

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

/**
One Client the demo names, beyond the World's anonymous tail.
*/
interface NamedClient {
	givenName: string;
	familyName: string;
	kind: 'birth' | 'postpartum';
	// Days from today, for a birth; undefined for postpartum care.
	dueInDays?: number;
}

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

type Session = { Cookie: string };

async function readOk(response: APIResponse, what: string): Promise<string> {
	const body = await response.text();
	if (!response.ok()) {
		throw new Error(`demo seed: ${what} answered ${response.status()}: ${body}`);
	}
	return body;
}

// One BFF call: through retryPastRateLimit, because the seed makes a few
// hundred writes from one address in a minute, and a refusal it causes
// itself is not a reason to stop.
async function send<T>(
	request: APIRequestContext,
	method: 'GET' | 'POST' | 'PUT' | 'PATCH',
	path: string,
	headers: Record<string, string>,
	data?: unknown
): Promise<T> {
	const response = await retryPastRateLimit(() => request.fetch(`${API_URL}${path}`, { method, headers, data }));
	const body = await readOk(response, `${method} ${path}`);
	return (body === '' ? {} : JSON.parse(body)) as T;
}

async function createIdentity(request: APIRequestContext, email: string): Promise<string> {
	const response = await request.post(`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`, {
		data: { email, password: DEMO_PASSWORD, returnSecureToken: true }
	});
	const { idToken } = JSON.parse(await readOk(response, `creating the emulator account for ${email}`));
	return idToken;
}

// The emulator's answer to a password sign-in: an ID token, or undefined
// when there is no such account -- the probe that tells a fresh stack from
// one already seeded.
async function signInWithPassword(request: APIRequestContext, email: string): Promise<string | undefined> {
	const response = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=fake-key`,
		{ data: { email, password: DEMO_PASSWORD, returnSecureToken: true } }
	);
	if (!response.ok()) return undefined;
	const { idToken } = await response.json();
	return idToken;
}

/**
The calendar day `days` after `day`, both YYYY-MM-DD.
*/
export function dayAfter(day: string, days: number): string {
	const date = new Date(`${day}T12:00:00Z`);
	date.setUTCDate(date.getUTCDate() + days);
	return date.toISOString().slice(0, 10);
}

/**
An instant `days` after `day`, at `hourUtc` o'clock UTC, as RFC 3339.
*/
function instantAfter(day: string, days: number, hourUtc: number): string {
	return `${dayAfter(day, days)}T${String(hourUtc).padStart(2, '0')}:00:00Z`;
}

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
	// Every Staff member but the Owner, by slug (her given name).
	staff: Record<string, DoulaSession>;
}

// Renata signs up, the way a Practice starts: an emulator account, then
// POST /api/staff/signup, which grants her all three roles.
async function signUpOwner(request: APIRequestContext): Promise<{ practiceId: string; owner: Session }> {
	const idToken = await createIdentity(request, addressOf(OWNER));
	const { givenName, familyName } = nameParts(OWNER);
	const { practiceId } = await send<{ practiceId: string }>(
		request,
		'POST',
		'/api/staff/signup',
		{ Authorization: `Bearer ${idToken}` },
		{
			practiceName: TYPICAL_PRACTICE_NAME,
			firstName: givenName,
			lastName: familyName,
			workState: OWNER.workState,
			timezone: 'America/New_York'
		}
	);
	return { practiceId, owner: await signIn(request, API_URL, idToken) };
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
	const { practiceId, owner } = await signUpOwner(request);
	const staff: Record<string, DoulaSession> = {};
	for (const member of INVITED) {
		staff[slugOf(member)] = await joinStaff(request, practiceId, owner, member);
	}
	// The pilot's founding grant (#439), sized from the roster just built:
	// three Credits a Staff member. An operator endpoint, so it takes the
	// stack's internal secret rather than a session.
	await send(request, 'POST', '/api/internal/billing/founding-grants', { 'X-Internal-Secret': WORKER_SECRET }, {
		practiceId,
		grantedBy: 'local demo seed (#1994)'
	});
	// A Contract takes its price from the rate for its kind (#967).
	await send(request, 'PUT', `/api/practices/${practiceId}/rates/birth`, owner, { amountCents: 150_000 });
	await send(request, 'PUT', `/api/practices/${practiceId}/rates/postpartum`, owner, { amountCents: 95_000 });
	return { practiceId, owner, staff };
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
		const isFinished = index % 4 === 3 && client.kind === 'postpartum';
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
		}
	}
}

interface NamedEngagement {
	client: NamedClient;
	clientId: string;
	engagementId: string;
	email: string;
}

// A named Client of Priya's: the record, with an address so the portal can
// reach her; the Engagement Request, which the Owner's own request
// approves at once (ADR-0017); and Priya's own first Visit, which attaches
// her, as provisionTail does for the tail.
async function addPriyaClient(
	request: APIRequestContext,
	roster: Roster,
	today: string,
	client: NamedClient
): Promise<NamedEngagement> {
	const { practiceId, owner, staff } = roster;
	const email = demoAddress(client.givenName, client.familyName);
	const { id: clientId } = await send<{ id: string }>(request, 'POST', `/api/practices/${practiceId}/clients`, owner, {
		givenName: client.givenName,
		familyName: client.familyName,
		email,
		override: false
	});
	const requested = await send<{ engagementId: string }>(
		request,
		'POST',
		`/api/practices/${practiceId}/clients/${clientId}/engagement-requests`,
		owner,
		client.dueInDays === undefined ? { kind: client.kind } : { kind: client.kind, dueDate: dayAfter(today, client.dueInDays) }
	);
	await send(request, 'POST', `/api/practices/${practiceId}/engagements/${requested.engagementId}/visits`, staff.priya.headers, {});
	return { client, clientId, engagementId: requested.engagementId, email };
}

// The Client's own portal session, through the Practice's invitation and
// her acceptance of it -- the first sign-in a real Client has (#90, #617).
// A Contract cannot be sent to a Client the portal has not invited, so
// even Grace, who has not signed in yet, holds an invitation.
async function invitePortal(request: APIRequestContext, roster: Roster, engagement: NamedEngagement): Promise<string> {
	const { inviteToken } = await send<{ inviteToken: string }>(
		request,
		'POST',
		`/api/practices/${roster.practiceId}/engagements/${engagement.engagementId}/portal-invite`,
		roster.owner
	);
	return inviteToken;
}

async function portalSessionFor(request: APIRequestContext, roster: Roster, engagement: NamedEngagement): Promise<Session> {
	const inviteToken = await invitePortal(request, roster, engagement);
	const accepted = await retryPastRateLimit(() =>
		request.post(`${API_URL}/api/portal/accept-invite`, { data: { inviteToken } })
	);
	await readOk(accepted, `${engagement.client.givenName} accepting her portal invitation`);
	return sessionCookieFrom(accepted, 'portal accept-invite');
}

// A Contract from the Practice's template, filled, sent, and -- unless
// `sign` is false -- signed by the Client in her portal. The fields the
// template leaves to a person are filled the way accessibility.e2e.ts
// fills them; the price is the rate's, and the BFF refuses a request that
// sets it.
async function contractFor(
	request: APIRequestContext,
	roster: Roster,
	today: string,
	engagement: NamedEngagement,
	portal: Session | undefined
): Promise<void> {
	const contractPath = `/api/practices/${roster.practiceId}/engagements/${engagement.engagementId}/contract`;
	const { values } = await send<{ values: Record<string, string> }>(request, 'POST', contractPath, roster.owner);
	const editable = Object.fromEntries(Object.entries(values).filter(([key]) => key !== 'price'));
	await send(request, 'PUT', contractPath, roster.owner, {
		values: {
			...editable,
			scope_of_service:
				engagement.client.kind === 'birth'
					? 'Two prenatal visits, on call from 37 weeks, continuous support in labor, and one postpartum visit.'
					: 'Six postpartum visits in the first eight weeks, daytime or overnight.',
			engagement_start_date: dayAfter(today, -30),
			engagement_end_date: dayAfter(today, 120)
		}
	});
	await send(request, 'POST', `${contractPath}/send`, roster.owner);
	if (portal === undefined) return;
	await send(request, 'POST', `/api/portal/engagements/${engagement.engagementId}/contract/sign`, portal, {
		fullLegalName: `${engagement.client.givenName} ${engagement.client.familyName}`,
		attestation: true
	});
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

// A Plan from the Practice's template, with its first text question
// answered and saved by whoever `headers` signs in as.
async function startAndSavePlan(
	request: APIRequestContext,
	roster: Roster,
	engagement: NamedEngagement,
	planType: 'care_plan' | 'birth_plan',
	headers: Record<string, string>,
	answer: string
): Promise<void> {
	const planPath = `/api/practices/${roster.practiceId}/engagements/${engagement.engagementId}/plans/${planType}`;
	const { fields } = await send<{ fields: { id: string; type: string }[] }>(request, 'POST', planPath, headers);
	const question = fields.find((field) => field.type === 'short_text' || field.type === 'long_text');
	await send(request, 'PUT', planPath, headers, { answers: question ? { [question.id]: answer } : {} });
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
	const messagesPath = (engagement: NamedEngagement) =>
		`/api/practices/${roster.practiceId}/engagements/${engagement.engagementId}/messages`;

	const camillePortal = await portalSessionFor(request, roster, camille);
	await contractFor(request, roster, today, camille, camillePortal);
	await raiseInvoice(request, roster, camille);
	// Her Birth Plan, from the Practice's own template, so the portal's
	// Birth plan screen has one to show -- and both Plans saved once, by
	// two different people, so the Engagement page's "Last edited" rows
	// have a save to name (#2104).
	await startAndSavePlan(request, roster, camille, 'birth_plan', priya.headers, 'Low lights, music, and her partner beside her.');
	await startAndSavePlan(request, roster, camille, 'care_plan', roster.owner, 'Iron supplements from 28 weeks.');
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
	await send(
		request,
		'PUT',
		`/api/practices/${roster.practiceId}/engagements/${mira.engagementId}/birth-outcome`,
		roster.staff.priya.headers,
		{ birthOutcome: 'loss', pregnancyEndedOn: dayAfter(today, -PREGNANCY_ENDED_DAYS_AGO) }
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
		send<{ offerId: string }>(
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
	const lena = roster.staff.lena;
	const { offerId } = await offer(births[0], lena.staffId, 45_000);
	await send(request, 'POST', `/api/practices/${roster.practiceId}/offers/${offerId}/accept`, lena.headers);
	await send(request, 'POST', `/api/practices/${roster.practiceId}/engagements/${births[0].engagementId}/visits`, lena.headers, {
		scheduledAt: instantAfter(today, 5, 17)
	});
	await offer(births[1], roster.staff.fern.staffId, 50_000);
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
	await seedPriyaBook(request, roster, today);
	await seedAfterALoss(request, roster, today);
	await seedOffers(request, roster, today, tail);
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

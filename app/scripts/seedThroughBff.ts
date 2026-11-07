// The writes both local seeds make: seed:demo (demo-practice.ts) and
// seed:scenario (scenarios.ts). Each goes through a BFF endpoint or the
// Auth emulator, the way the e2e helpers do, so the rows and their audit
// trail are the ones the product itself makes.
import type { APIRequestContext, APIResponse } from '@playwright/test';
import { sessionCookieFrom, signIn } from '../e2e/auth';
import { API_URL, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from '../e2e/ports';
import { retryPastRateLimit } from '../e2e/rateLimit';

/**
 * The one password every seeded account signs in with. Dev-only and fake on
 * purpose: it exists in the local Auth emulator and nowhere else, and the
 * emulator is wiped each time the stack stops. Fifteen characters or more,
 * the Staff rule (#1538).
 */
export const DEMO_PASSWORD = 'demo-password-local-only';

// The domain the simulation already gives a simulated person's address
// (docs/simulation/environment.md), so a log says at a glance that the
// address is nobody's. Mail to it goes to the sandbox mailbox only.
const DEMO_DOMAIN = 'sim.doula.cloud';

const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;

export function demoAddress(givenName: string, familyName: string): string {
	return `${givenName}.${familyName}@${DEMO_DOMAIN}`.toLowerCase();
}

export type Session = { Cookie: string };

/**
A Practice and the session that writes to it as its Owner.
*/
export interface PracticeSession {
	practiceId: string;
	owner: Session;
}

export async function readOk(response: APIResponse, what: string): Promise<string> {
	const body = await response.text();
	if (!response.ok()) {
		throw new Error(`seed: ${what} answered ${response.status()}: ${body}`);
	}
	return body;
}

// One BFF call: through retryPastRateLimit, because a seed makes a few
// hundred writes from one address in a minute, and a refusal it causes
// itself is not a reason to stop.
export async function send<T>(
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

export async function createIdentity(request: APIRequestContext, email: string): Promise<string> {
	const response = await request.post(`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`, {
		data: { email, password: DEMO_PASSWORD, returnSecureToken: true }
	});
	const { idToken } = JSON.parse(await readOk(response, `creating the emulator account for ${email}`));
	return idToken;
}

// The emulator's answer to a password sign-in: an ID token, or undefined
// when there is no such account.
export async function signInWithPassword(request: APIRequestContext, email: string): Promise<string | undefined> {
	const response = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=fake-key`,
		{ data: { email, password: DEMO_PASSWORD, returnSecureToken: true } }
	);
	if (!response.ok()) return undefined;
	const { idToken } = await response.json();
	return idToken;
}

/**
The fields POST /api/staff/signup takes, with the address her account has.
*/
export interface Founder {
	email: string;
	practiceName: string;
	givenName: string;
	familyName: string;
	workState: string;
}

/**
A Practice's founding Owner, as signup made her.
*/
export interface FoundedPractice extends PracticeSession {
	staffId: string;
}

// A Practice starts the way a real one does: an emulator account, then
// POST /api/staff/signup, which grants the Owner all three roles.
export async function signUpPractice(request: APIRequestContext, founder: Founder): Promise<FoundedPractice> {
	const idToken = await createIdentity(request, founder.email);
	const { practiceId, staffId } = await send<{ practiceId: string; staffId: string }>(
		request,
		'POST',
		'/api/staff/signup',
		{ Authorization: `Bearer ${idToken}` },
		{
			practiceName: founder.practiceName,
			firstName: founder.givenName,
			lastName: founder.familyName,
			workState: founder.workState,
			timezone: 'America/New_York'
		}
	);
	return { practiceId, staffId, owner: await signIn(request, API_URL, idToken) };
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
export function instantAfter(day: string, days: number, hourUtc: number): string {
	return `${dayAfter(day, days)}T${String(hourUtc).padStart(2, '0')}:00:00Z`;
}

/**
One Client a seed names.
*/
export interface NamedClient {
	givenName: string;
	familyName: string;
	kind: 'birth' | 'postpartum';
	// Days from today, for a birth; undefined for postpartum care.
	dueInDays?: number;
}

export interface NamedEngagement {
	client: NamedClient;
	clientId: string;
	engagementId: string;
	email: string;
}

/**
 * A Client's record, with an address so the portal can reach her, and her
 * Engagement Request, which a request from someone with approval authority
 * approves at once (ADR-0017). `doulaStaffId` names her Doula on the
 * request; without it the Engagement starts with no Doula.
 */
export async function addClient(
	request: APIRequestContext,
	practice: PracticeSession,
	today: string,
	client: NamedClient,
	options: { email?: string; doulaStaffId?: string; override?: boolean } = {}
): Promise<NamedEngagement> {
	const email = options.email ?? demoAddress(client.givenName, client.familyName);
	const { id: clientId } = await send<{ id: string }>(request, 'POST', `/api/practices/${practice.practiceId}/clients`, practice.owner, {
		givenName: client.givenName,
		familyName: client.familyName,
		email,
		override: options.override ?? false
	});
	const { engagementId } = await send<{ engagementId: string }>(
		request,
		'POST',
		`/api/practices/${practice.practiceId}/clients/${clientId}/engagement-requests`,
		practice.owner,
		{
			kind: client.kind,
			...(client.dueInDays !== undefined && { dueDate: dayAfter(today, client.dueInDays) }),
			...(options.doulaStaffId !== undefined && { doulaStaffId: options.doulaStaffId })
		}
	);
	return { client, clientId, engagementId, email };
}

// The Client's own portal session, through the Practice's invitation and
// her acceptance of it -- the first sign-in a real Client has (#90, #617).
// A Contract cannot be sent to a Client the portal has not invited.
export async function invitePortal(request: APIRequestContext, practice: PracticeSession, engagement: NamedEngagement): Promise<string> {
	const { inviteToken } = await send<{ inviteToken: string }>(
		request,
		'POST',
		`/api/practices/${practice.practiceId}/engagements/${engagement.engagementId}/portal-invite`,
		practice.owner
	);
	return inviteToken;
}

export async function portalSessionFor(
	request: APIRequestContext,
	practice: PracticeSession,
	engagement: NamedEngagement
): Promise<Session> {
	const inviteToken = await invitePortal(request, practice, engagement);
	const accepted = await retryPastRateLimit(() =>
		request.post(`${API_URL}/api/portal/accept-invite`, { data: { inviteToken } })
	);
	await readOk(accepted, `${engagement.client.givenName} accepting her portal invitation`);
	return sessionCookieFrom(accepted, 'portal accept-invite');
}

// A Contract from the Practice's template, filled and saved as a Draft;
// sent unless `send` is false; and signed by the Client in her portal when
// `portal` is given. The fields the template leaves to a person are filled
// the way accessibility.e2e.ts fills them; the price is the rate's, and the
// BFF refuses a request that sets it.
export async function contractFor(
	request: APIRequestContext,
	practice: PracticeSession,
	today: string,
	engagement: NamedEngagement,
	portal: Session | undefined,
	options: { send?: boolean } = {}
): Promise<void> {
	const contractPath = `/api/practices/${practice.practiceId}/engagements/${engagement.engagementId}/contract`;
	const { values } = await send<{ values: Record<string, string> }>(request, 'POST', contractPath, practice.owner);
	const editable = Object.fromEntries(Object.entries(values).filter(([key]) => key !== 'price'));
	await send(request, 'PUT', contractPath, practice.owner, {
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
	if (options.send === false) return;
	await send(request, 'POST', `${contractPath}/send`, practice.owner);
	if (portal === undefined) return;
	await send(request, 'POST', `/api/portal/engagements/${engagement.engagementId}/contract/sign`, portal, {
		fullLegalName: `${engagement.client.givenName} ${engagement.client.familyName}`,
		attestation: true
	});
}

type PlanAnswers = Record<string, string | string[] | boolean>;

// A Plan from the Practice's template, started and saved by whoever
// `headers` signs in as, with each answer in `answers` keyed by the
// question it answers, worded as the template words it (#2137). An answer
// keyed by its position put "Iron supplements from 28 weeks." under "Who
// will be present for support?", so a label the template does not hold
// stops the seed rather than putting an answer beside the wrong question.
// `saves` saves it again with each later set of answers, merged over the
// first, so its history holds more than one save.
export async function startAndSavePlan(
	request: APIRequestContext,
	practice: PracticeSession,
	engagement: NamedEngagement,
	planType: 'care_plan' | 'birth_plan',
	headers: Record<string, string>,
	answers: PlanAnswers,
	saves: readonly PlanAnswers[] = []
): Promise<void> {
	const planPath = `/api/practices/${practice.practiceId}/engagements/${engagement.engagementId}/plans/${planType}`;
	const { fields } = await send<{ fields: { id: string; label: string }[] }>(request, 'POST', planPath, headers);
	const byId: PlanAnswers = {};
	const everySave = [answers, ...saves];
	for (const each of everySave) {
		for (const [label, answer] of Object.entries(each)) {
			const field = fields.find((candidate) => candidate.label === label);
			if (!field) {
				const asked = fields.map((candidate) => `"${candidate.label}"`).join(', ');
				throw new Error(`seed: the ${planType} template has no question "${label}"; it asks ${asked}`);
			}
			byId[field.id] = answer;
		}
		await send(request, 'PUT', planPath, headers, { answers: { ...byId } });
	}
}

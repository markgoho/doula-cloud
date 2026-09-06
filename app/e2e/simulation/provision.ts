// The World's two provisioning primitives (#822, under map #779/#759):
// standUpRidgeline, which creates a Practice that predates the run and
// writes no friction-log entry for any of it, and provisionTail, which
// creates a slice of the anonymous Client tail world.ts's describeWorld
// lays out. Both call the ordinary authenticated API -- the same
// signIn/seedFoundingOwner primitives dozens of other e2e specs already
// use to provision their own fixtures -- never raw SQL, and neither ever
// calls observedAct: nothing here is a walked act.
import { mkdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { type APIRequestContext, expect } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from '../ports';
import { MAILBOX_URL, readStaffInviteToken } from '../stack';
import { signInEnrolled } from '../mfa';
import { seedFoundingOwner } from '../staffSignup';
import type { SeededClient, WorldDescription } from './world';

const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;

async function readBody(response: { ok(): boolean; status(): number; text(): Promise<string> }, context: string): Promise<string> {
	const body = await response.text();
	expect(response.ok(), `${context}: ${response.status()} ${body}`).toBe(true);
	return body;
}

export interface RidgelineLenaAccount {
	staffId: string;
	// Her own credentials, not a session -- #823 needs to mint a fresh
	// ID token from these when it walks her later Rooted acceptance
	// under the same Firebase identity, which is the whole point of
	// standing Ridgeline up first (worlds/rooted-birth-collective.md's
	// "Lena's Ridgeline account predates her Rooted invitation").
	email: string;
	password: string;
	localId: string;
}

export interface RidgelineStandUpResult {
	practiceId: string;
	deborahStaffId: string;
	// Deborah has no journey and no friction log (worlds/rooted-birth-collective.md),
	// so nothing re-authenticates her -- this is the one session she
	// ever holds, handed back for a caller that needs to read Ridgeline's
	// own roster (this module's own test does exactly that).
	ownerHeaders: { Cookie: string };
	lena: RidgelineLenaAccount;
}

const LENA_PASSWORD = 'ridgeline-password-123';

// standUpRidgeline creates Ridgeline Doula Group, Deborah Ridge as its
// founding Owner, and Lena Vasquez as a real, usable contractor Doula
// there -- a real Firebase identity and a real staff.identity_uid --
// before Rooted ever invites her. Every call here is the ordinary
// authenticated API a walked spec would use too; what makes this
// provisioning rather than a walk is that nothing is driven through a
// Page and nothing is passed to observedAct, so no friction-log entry
// is ever produced for it, exactly as the ticket requires.
export async function standUpRidgeline(request: APIRequestContext): Promise<RidgelineStandUpResult> {
	// Labels the mailbox before Lena's invitation is sent, so her inbox
	// never shows this message under whatever label a later jump leaves
	// behind -- the run's own clock has not started yet (#762, #764).
	const labelled = await request.post(`${MAILBOX_URL}/api/clock`, { data: { label: 'before day zero' } });
	expect(labelled.ok(), `standUpRidgeline: labelling the mailbox failed: ${labelled.status()}`).toBe(true);

	const owner = await seedFoundingOwner(request, {
		practiceName: 'Ridgeline Doula Group',
		staffName: 'Deborah Ridge',
		workState: 'NY'
	});
	// staffauth.Middleware refuses any Practice-scoped call from an
	// unenrolled Owner (#606) regardless of the Practice's own MFA
	// switch, so Deborah needs a second factor before she can invite
	// anyone -- signInEnrolled is mfa.ts's fixture-only route to a
	// session carrying that claim.
	const ownerHeaders = await signInEnrolled(request, owner.idToken, owner.localId);

	const lenaEmail = 'lena-vasquez@sim.doula.cloud';
	const invited = await request.post(`${API_URL}/api/practices/${owner.practiceId}/staff/invitations`, {
		headers: ownerHeaders,
		data: { email: lenaEmail, roles: ['doula'], employmentType: 'contractor' }
	});
	const { invitationId } = JSON.parse(await readBody(invited, 'standUpRidgeline: inviting Lena'));

	// readStaffInviteToken reads the token straight off the pending
	// outbox row (stack.ts). worlds/rooted-birth-collective.md's
	// objection to that shortcut ("it skips the act") is about a
	// *walked* invitation; Ridgeline's is explicitly not one.
	const inviteToken = readStaffInviteToken(invitationId);

	const signedUp = await request.post(`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`, {
		data: { email: lenaEmail, password: LENA_PASSWORD, returnSecureToken: true }
	});
	expect(signedUp.ok(), `standUpRidgeline: creating Lena's Identity Platform account failed: ${signedUp.status()}`).toBe(true);
	const { idToken: lenaIdToken, localId } = await signedUp.json();

	const accepted = await request.post(`${API_URL}/api/staff/accept-invite`, {
		headers: { Authorization: `Bearer ${lenaIdToken}` },
		data: { inviteToken, name: 'Lena Vasquez', workState: 'NY' }
	});
	const { staffId: lenaStaffId } = JSON.parse(await readBody(accepted, "standUpRidgeline: Lena accepting her invitation"));

	return {
		practiceId: owner.practiceId,
		deborahStaffId: owner.staffId,
		ownerHeaders,
		lena: { staffId: lenaStaffId, email: lenaEmail, password: LENA_PASSWORD, localId }
	};
}

export interface DoulaSession {
	staffId: string;
	headers: { Cookie: string };
}

export interface ProvisionedClient extends SeededClient {
	clientId: string;
	engagementId: string;
}

// provisionTail creates one slice of world.ts's anonymous tail: a Client,
// an Engagement Request that collapses into an approved Engagement in
// the same call (ADR-0017's solo-approval collapse, since an Owner or
// Admin session always holds approval authority), and one Visit by the
// assigned doula to attach her (staffauth.AttachingWrite's accrual --
// the only mechanism that reaches an Engagement for an employee Doula
// without a granted Offer). Deliberately never called with the whole 43
// at once: each approval spends a Credit
// (billing.ConsumeCredit/engagementrequest.approve), and Rooted's World
// requires Renata to run dry on Credit 46 while a *walked* Engagement is
// what trips it (the P4 probe, week 2) -- #823 owns interleaving this
// against the 15 it walks, this function only has to be safe to call in
// slices.
//
// doulaSessions supplies, for each doula slug this slice's clients name,
// her own authenticated session -- her Invitation must already be
// walked and accepted (#823's job) before any of her tail Clients can be
// provisioned, so this function never creates a doula account itself.
export async function provisionTail(
	request: APIRequestContext,
	practiceId: string,
	ownerHeaders: { Cookie: string },
	doulaSessions: Readonly<Record<string, DoulaSession>>,
	clients: readonly SeededClient[]
): Promise<ProvisionedClient[]> {
	const provisioned: ProvisionedClient[] = [];

	for (const client of clients) {
		const doula = doulaSessions[client.assignedDoulaSlug];
		if (!doula) {
			throw new Error(
				`provisionTail: no session supplied for doula '${client.assignedDoulaSlug}' -- her Invitation must be walked and accepted before ${client.slug} can be provisioned`
			);
		}

		const created = await request.post(`${API_URL}/api/practices/${practiceId}/clients`, {
			headers: ownerHeaders,
			data: { givenName: client.givenName, familyName: client.familyName, override: false }
		});
		const { id: clientId } = JSON.parse(await readBody(created, `provisionTail: creating ${client.slug}`));

		const requested = await request.post(`${API_URL}/api/practices/${practiceId}/clients/${clientId}/engagement-requests`, {
			headers: ownerHeaders,
			data: client.dueDate ? { kind: client.kind, dueDate: client.dueDate } : { kind: client.kind }
		});
		const requestedBody = JSON.parse(await readBody(requested, `provisionTail: requesting ${client.slug}'s Engagement`));
		if (requestedBody.state !== 'approved' || !requestedBody.engagementId) {
			throw new Error(
				`provisionTail: ${client.slug}'s request did not collapse into an approved Engagement (state ${requestedBody.state}) -- ownerHeaders must belong to an Owner or Admin`
			);
		}
		const engagementId: string = requestedBody.engagementId;

		const visit = await request.post(`${API_URL}/api/practices/${practiceId}/engagements/${engagementId}/visits`, {
			headers: doula.headers,
			data: {}
		});
		await readBody(visit, `provisionTail: attaching ${client.assignedDoulaSlug} to ${client.slug}'s Engagement`);

		provisioned.push({ ...client, clientId, engagementId });
	}

	return provisioned;
}

// writeWorldRecord is the provisioned-vs-walked record the ticket's own
// key interfaces ask for: everything a run README needs to state how
// many of a run's Clients were provisioned rather than walked, without
// this module ever writing the run README itself -- that stays #823's.
export function writeWorldRecord(runsRoot: string, runId: string, description: WorldDescription, provisioned: readonly ProvisionedClient[]): string {
	const runDirectory = path.join(runsRoot, runId);
	mkdirSync(runDirectory, { recursive: true });
	const filePath = path.join(runDirectory, 'world.json');
	const record = {
		seed: description.seed,
		summary: description.summary,
		stagedArrivals: description.stagedArrivals,
		allocations: description.allocations,
		provisionedClients: provisioned.map((client) => ({
			slug: client.slug,
			givenName: client.givenName,
			familyName: client.familyName,
			kind: client.kind,
			dueDate: client.dueDate,
			assignedDoulaSlug: client.assignedDoulaSlug,
			clientId: client.clientId,
			engagementId: client.engagementId
		}))
	};
	writeFileSync(filePath, JSON.stringify(record, undefined, 2) + '\n');
	return filePath;
}

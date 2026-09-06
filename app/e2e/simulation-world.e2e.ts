// The World stands up (#822, under map #759): describeWorld's seeded
// reproducibility, standUpRidgeline's pre-run Practice, and a slice of
// provisionTail's anonymous Client tail. Follows simulation-harness.e2e.ts's
// pattern -- a scratch runs root under app/test-results/, never
// docs/simulation/runs/, since this proves the primitives, not a real run.
import { readFileSync, rmSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from '@playwright/test';
import { sessionCookieFrom } from './auth';
import { signInEnrolled } from './mfa';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { provisionTail, standUpRidgeline, writeWorldRecord } from './simulation/provision';
import { describeWorld, ROOTED_TAIL_TOTAL, type SeededClient } from './simulation/world';
import { readStaffInviteToken } from './stack';
import { seedFoundingOwner } from './staffSignup';

const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const RUNS_ROOT = path.join(path.dirname(fileURLToPath(import.meta.url)), 'test-results', 'simulation-world-rehearsal');
const RUN_ID = 'rehearsal-1';

test.describe.serial('The World stands up: description, Ridgeline, the tail', () => {
	test.afterAll(() => {
		rmSync(RUNS_ROOT, { recursive: true, force: true });
	});

	test('the same seed describes a deep-equal World, and a different seed reshuffles the tail', () => {
		const first = describeWorld(4_102_027);
		const second = describeWorld(4_102_027);
		expect(second).toEqual(first);
		expect(first.tailClients).toHaveLength(ROOTED_TAIL_TOTAL);
		expect(first.summary).toEqual({
			rooted: { total: 58, walked: 15, provisioned: ROOTED_TAIL_TOTAL },
			okonkwo: { total: 5, walked: 5, provisioned: 0 }
		});

		const differentSeed = describeWorld(7);
		expect(differentSeed.tailClients).not.toEqual(first.tailClients);
	});

	test('standUpRidgeline creates a real, usable contractor Membership for Lena, with no log entry written for any of it', async ({ request }) => {
		const result = await standUpRidgeline(request);

		// A real account: she can sign in with the credentials this
		// function handed back, exactly as #823 will need to when it
		// walks her Rooted acceptance later under the same identity.
		const signedIn = await request.post(`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=fake-key`, {
			data: { email: result.lena.email, password: result.lena.password, returnSecureToken: true }
		});
		expect(signedIn.ok()).toBe(true);
		const { localId } = await signedIn.json();
		expect(localId).toBe(result.lena.localId);

		// A real Membership: Deborah's own roster shows Lena as a doula
		// contractor. standUpRidgeline's own source never constructs a
		// PersonaLog or calls observedAct -- there is no log for any of
		// this to land in, which is what "no entry is written for it"
		// means here, structurally rather than by a runtime check.
		const roster = await request.get(`${API_URL}/api/practices/${result.practiceId}/staff`, {
			headers: result.ownerHeaders
		});
		expect(roster.ok()).toBe(true);
		const { members } = await roster.json();
		const lenaMembership = members.find((member: { staffId: string }) => member.staffId === result.lena.staffId);
		expect(lenaMembership).toMatchObject({ roles: ['doula'], employmentType: 'contractor' });
	});

	test('provisionTail creates a real Client, Engagement and doula Attachment for a slice of the tail, without a Credit purchase', async ({ request }) => {
		const owner = await seedFoundingOwner(request, { practiceName: 'Tail Rehearsal Practice', staffName: 'Test Owner', workState: 'NY' });
		// An Owner always needs a second factor to make a Practice-scoped
		// call (#606, api/internal/staffauth/middleware.go), regardless of
		// the Practice's own MFA switch.
		const ownerHeaders = await signInEnrolled(request, owner.idToken, owner.localId);

		// A plain employee Doula, distinct from the founding Owner --
		// staffauth.AttachingWrite never accrues an Owner or Admin to an
		// Engagement she writes on, so the tail's own doulas must be
		// ordinary Staff, exactly as Rooted's Extras are.
		const doulaEmail = `test-doula-${Date.now()}@example.com`;
		const invited = await request.post(`${API_URL}/api/practices/${owner.practiceId}/staff/invitations`, {
			headers: ownerHeaders,
			data: { email: doulaEmail, roles: ['doula'], employmentType: 'employee' }
		});
		const { invitationId } = JSON.parse(await invited.text());
		const inviteToken = readStaffInviteToken(invitationId);
		const signedUp = await request.post(`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`, {
			data: { email: doulaEmail, password: 'password123', returnSecureToken: true }
		});
		const { idToken: doulaIdToken } = await signedUp.json();
		const accepted = await request.post(`${API_URL}/api/staff/accept-invite`, {
			headers: { Authorization: `Bearer ${doulaIdToken}` },
			data: { inviteToken, name: 'Test Doula', workState: 'NY' }
		});
		const doulaHeaders = sessionCookieFrom(accepted, 'accept-invite');
		const { staffId: doulaStaffId } = JSON.parse(await accepted.text());

		const clients: SeededClient[] = [
			{ slug: 'rehearsal-tail-1', givenName: 'Rehearsal', familyName: 'Tailone', kind: 'postpartum', dueDate: '', assignedDoulaSlug: 'test-doula' },
			{ slug: 'rehearsal-tail-2', givenName: 'Rehearsal', familyName: 'Tailtwo', kind: 'birth', dueDate: '2027-03-15', assignedDoulaSlug: 'test-doula' }
		];

		const provisioned = await provisionTail(
			request,
			owner.practiceId,
			ownerHeaders,
			{ 'test-doula': { staffId: doulaStaffId, headers: doulaHeaders } },
			clients
		);

		expect(provisioned).toHaveLength(2);
		for (const client of provisioned) {
			expect(client.clientId).toBeTruthy();
			expect(client.engagementId).toBeTruthy();
		}

		const description = describeWorld(4_102_027);
		const filePath = writeWorldRecord(RUNS_ROOT, RUN_ID, description, provisioned);
		const record = JSON.parse(readFileSync(filePath, 'utf8'));

		expect(record.seed).toBe(description.seed);
		expect(record.summary.rooted.provisioned).toBe(ROOTED_TAIL_TOTAL);
		expect(record.provisionedClients).toHaveLength(2);
		expect(record.provisionedClients[1]).toMatchObject({ slug: 'rehearsal-tail-2', kind: 'birth', dueDate: '2027-03-15' });
	});
});


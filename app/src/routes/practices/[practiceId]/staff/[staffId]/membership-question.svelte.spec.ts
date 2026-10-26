import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { captureLanding } from '#lib/testOutcome.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import RolesPage from './roles/+page.svelte';
import EmploymentTypePage from './employment-type/+page.svelte';
import { toPageState, type RouteFixture, type RouteVariant } from '../../../../routeFixture.js';
import { asAdmin as rolesAsAdmin, fixture as rolesFixture } from './roles/page.fixture.js';
import { asAdmin as employmentAsAdmin, fixture as employmentFixture } from './employment-type/page.fixture.js';
import { roster } from '../page.fixture.js';

if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The roles and employment type question pages (#2041), one spec for
 * both: they are one component, `MembershipQuestion`, asked two ways.
 * The page, the session and the roster are each page's own fixture.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', async () => ({
	apiFetchWithSession,
	...(await vi.importActual<typeof import('#lib/apiErrorMessage.js')>('#lib/apiErrorMessage.js'))
}));

const [owner] = roster.members;

interface Options {
	fixture: RouteFixture;
	variant?: RouteVariant;
	saveResponse?: Response;
	rosterResponse?: Response;
}

async function setup({ fixture, variant, saveResponse, rosterResponse }: Options) {
	Object.assign(pageState, toPageState(variant ? { ...fixture, ...variant } : fixture));
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (init?.method === 'PATCH') return Promise.resolve(saveResponse ?? jsonResponse({}));
		return Promise.resolve(rosterResponse ?? fixture.respond!(path));
	});
	await render(fixture === rolesFixture ? RolesPage : EmploymentTypePage, {});
}

// The body of the one PATCH sent, parsed.
function savedMembership(): unknown {
	const call = apiFetchWithSession.mock.calls.find((each: unknown[]) => (each[1] as RequestInit | undefined)?.method === 'PATCH');
	return call ? JSON.parse(String((call[1] as RequestInit).body)) : undefined;
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
});

describe('the roles question', () => {
	it('asks the question as the page heading, opens on the stored roles, and goes back to the person', async () => {
		await setup({ fixture: rolesFixture });

		await expect.element(testPage.getByRole('heading', { level: 1, name: `Which roles does ${owner!.name} have?` })).toBeVisible();
		await expect.element(testPage.getByRole('checkbox', { name: 'Owner' })).toBeChecked();
		await expect.element(testPage.getByRole('checkbox', { name: 'Admin' })).toBeChecked();
		await expect.element(testPage.getByRole('checkbox', { name: 'Doula' })).not.toBeChecked();
		await expect
			.element(testPage.getByRole('link', { name: 'Back' }))
			.toHaveAttribute('href', `/practices/practice-1/staff/${owner!.staffId}`);
	});

	it('saves the roles with the employment type as it stands, and says what changed on the way back', async () => {
		await setup({ fixture: rolesFixture });
		const landing = captureLanding(goto);

		await testPage.getByRole('checkbox', { name: 'Doula' }).click();
		await testPage.getByRole('button', { name: 'Save' }).click();

		await vi.waitFor(() => expect(landing.href).toBe(`/practices/practice-1/staff/${owner!.staffId}`));
		expect(savedMembership()).toEqual({ roles: ['owner', 'admin', 'doula'], employmentType: 'employee' });
		expect(landing.message).toBe(`${owner!.name}'s roles are now Owner, Admin, Doula.`);
	});

	it('refuses an empty set before it asks the server, and says so at the group', async () => {
		await setup({ fixture: rolesFixture });

		await testPage.getByRole('checkbox', { name: 'Owner' }).click();
		await testPage.getByRole('checkbox', { name: 'Admin' }).click();
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Select at least one role' })).toBeVisible();
		await expect.element(testPage.getByRole('alert').filter({ hasText: 'Select at least one role' }).last()).toBeVisible();
		expect(savedMembership()).toBeUndefined();
	});

	it("shows the server's refusal and stays on the question", async () => {
		await setup({
			fixture: rolesFixture,
			saveResponse: jsonResponse({ code: 'FAILED_PRECONDITION', message: 'a practice must keep at least one Owner' }, 409)
		});

		await testPage.getByRole('checkbox', { name: 'Owner' }).click();
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByText('a practice must keep at least one Owner')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('tells an Admin who can change roles, and reads nothing', async () => {
		await setup({ fixture: rolesFixture, variant: rolesAsAdmin });

		await expect.element(testPage.getByText('Only a practice owner can change roles.', { exact: false })).toBeVisible();
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});

	it('says when the person is not on the roster', async () => {
		await setup({ fixture: rolesFixture, rosterResponse: jsonResponse({ ...roster, members: [] }) });

		await expect.element(testPage.getByText('That Staff member is not on this practice roster.')).toBeVisible();
	});
});

describe('the employment type question', () => {
	it('opens on the stored answer, and saves the new one with the roles as they stand', async () => {
		await setup({ fixture: employmentFixture });
		const landing = captureLanding(goto);

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: `Is ${owner!.name} an employee or a contractor?` }))
			.toBeVisible();
		await expect.element(testPage.getByRole('radio', { name: 'Employee' })).toBeChecked();
		await testPage.getByRole('radio', { name: 'Contractor' }).click();
		await testPage.getByRole('button', { name: 'Save' }).click();

		await vi.waitFor(() => expect(landing.href).toBe(`/practices/practice-1/staff/${owner!.staffId}`));
		expect(savedMembership()).toEqual({ roles: ['owner', 'admin'], employmentType: 'contractor' });
		expect(landing.message).toBe(`${owner!.name}'s employment type is now Contractor.`);
	});

	it('tells an Admin who can change it', async () => {
		await setup({ fixture: employmentFixture, variant: employmentAsAdmin });

		await expect.element(testPage.getByText('Only a practice owner can change employment type.', { exact: false })).toBeVisible();
	});
});

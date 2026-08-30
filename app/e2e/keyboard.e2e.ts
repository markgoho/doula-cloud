import { expect, test, type Locator, type Page } from '@playwright/test';
import { seedClient, seedPortalClient, PORTAL_CLIENT_PASSWORD } from './portalClient';

/**
 * The half of the accessibility gate axe cannot see (#447): whether a
 * task can be *completed* with no pointer at all. axe reads one rendered
 * page and can tell you a control has an accessible name; it cannot tell
 * you the control is reachable in a sane order, or that pressing Enter on
 * it does what pressing it with a mouse does.
 *
 * So this spec uses no `.click()`, no `.fill()`, and no `.focus()` --
 * only `page.keyboard`. `.fill()` in particular would defeat the point:
 * it sets a value without the field ever being focused, which is exactly
 * the step a keyboard user cannot skip.
 *
 * The first walk is Stages 1 and 2 of docs/journeys/practice-owner.md --
 * Renata signs in, and invites a Doula to her Practice. Chosen because it
 * crosses the most surfaces in one task: the signed-out shell, the Staff
 * shell's nav, a record list, and a form that writes. The second walk
 * (#516) covers what the first has no example of: a modal confirmation
 * and a radio group.
 */

// Generous, because the point is reachability rather than a frozen tab
// order: the shell's own chrome is a dozen stops before the page starts,
// and freezing that count would make every nav change a failing test for
// no accessibility reason. What is asserted about *order* is asserted
// directly -- the skip link is first, the password follows the email.
const MAX_TABS = 40;

async function describeFocus(page: Page): Promise<string> {
	return page.evaluate(() => {
		const element = document.activeElement;
		if (!element || element === document.body) {
			return 'nothing (focus is on the body)';
		}
		const name =
			element.getAttribute('aria-label') ??
			element.getAttribute('name') ??
			element.textContent?.trim() ??
			'';
		return `<${element.tagName.toLowerCase()}> ${name}`.slice(0, 120);
	});
}

/**
 * Presses Tab until target holds focus, and returns how many it took.
 */
async function tabTo(page: Page, target: Locator, description: string): Promise<number> {
	// This is a client-rendered SPA, so a Tab pressed before the route has
	// painted is a press into an empty document -- it would spend the
	// budget below and report a control as unreachable when it was only
	// late.
	await target.waitFor({ state: 'visible' });
	for (let pressed = 1; pressed <= MAX_TABS; pressed += 1) {
		await page.keyboard.press('Tab');
		if (await target.evaluate((element) => element === document.activeElement)) {
			return pressed;
		}
	}
	throw new Error(
		`${description} is not reachable by keyboard: ${MAX_TABS} Tab presses left focus on ${await describeFocus(page)}`
	);
}

/**
 * Stage 1 of every walk below -- signing in with nothing but the keyboard.
 * The skip link's position and the password's adjacency to the email are
 * asserted here rather than in a caller, because they are the same defect
 * whichever journey meets them first.
 */
async function signInByKeyboard(page: Page, email: string, practiceId: string) {
	await page.goto('/login');
	await expect(page.getByRole('heading', { level: 1, name: 'Log in' })).toBeVisible();

	await page.keyboard.press('Tab');
	await expect(
		page.getByRole('link', { name: 'Skip to main content' }),
		'the skip link must be the first stop, or it skips nothing'
	).toBeFocused();

	await tabTo(page, page.getByLabel('Email'), 'the Email field on /login');
	await page.keyboard.type(email);

	// Asserted as an adjacency rather than found by tabTo: a password that
	// does not follow its email is a real defect, not a preference.
	await page.keyboard.press('Tab');
	await expect(page.getByLabel('Password')).toBeFocused();
	await page.keyboard.type(PORTAL_CLIENT_PASSWORD);

	await tabTo(page, page.getByRole('button', { name: 'Log in' }), 'the Log in button');
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));
}

test('Renata signs in and invites a Doula, with no pointer at any step', async ({
	page,
	request
}) => {
	// A Practice with a Client already in it: Renata's journey opens on a
	// working Practice, not an empty one, and the hub renders its primary
	// region rather than its empty state.
	const seeded = await seedPortalClient(request, 'Riverside Doulas');
	const { practiceId } = seeded;
	const invited = `doula-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;

	// Stage 1 -- sign in.
	await signInByKeyboard(page, seeded.staffEmail, practiceId);

	// Stage 2 -- reach the roster through the shell's own nav, and invite.
	await tabTo(
		page,
		page.getByRole('navigation', { name: 'Practice' }).getByRole('link', { name: 'Staff' }),
		'the Staff link in the Practice nav'
	);
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/staff$`));

	await tabTo(
		page,
		page.getByRole('main').getByRole('link', { name: 'Invite a Staff member' }),
		'the Invite a Staff member link on the roster'
	);
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/invite$`));

	await tabTo(page, page.getByLabel('Their email'), 'the email field on the invite form');
	await page.keyboard.type(invited);

	await tabTo(page, page.getByRole('button', { name: 'Send invite' }), 'the Send invite button');
	await page.keyboard.press('Enter');

	// Done looks like: the Invitation is out, and the screen says so
	// without anyone having touched a pointing device.
	await expect(
		page.getByText(`An email with a link to join is on its way to ${invited}`)
	).toBeVisible();
});

/*
 * The second walk (#516): the three screens under clients/[clientId],
 * which between them hold the two controls axe cannot judge -- a modal
 * confirmation, and a radio group that is the whole question. Both are
 * reached the way a keyboard user reaches them, from the Client's own
 * detail hub.
 *
 * The rename to "Jane" is what makes the confirmation appear at all:
 * ADR-0017's match query is a case-insensitive substring on the three
 * name columns, so saving Pat under the given name of the Practice's
 * other Client refuses with a 409 and nothing is written. That is the
 * only door to the override dialog, so the walk has to go through it.
 */
test('A doula edits a Client and starts new work, with no pointer at any step', async ({
	page,
	request
}) => {
	const seeded = await seedPortalClient(request, 'Riverside Doulas');
	const { practiceId, clientId, staffHeaders } = seeded;

	// The record the rename below collides with. Plain, no portal account:
	// it exists only to be matched.
	await seedClient(request, practiceId, staffHeaders, {
		givenName: 'Jane',
		familyName: 'Smith',
		email: `jane-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`
	});

	await signInByKeyboard(page, seeded.staffEmail, practiceId);

	const hubURL = `/practices/${practiceId}/clients/${clientId}`;
	await page.goto(hubURL);

	// Stage 1 -- reach the edit form from the hub.
	await tabTo(
		page,
		page.getByRole('main').getByRole('link', { name: 'Edit', exact: true }),
		'the Edit link on the Client detail hub'
	);
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(new RegExp(`${hubURL}/edit$`));

	// Stage 2 -- retype the given name. ControlOrMeta so the selection
	// works on both a developer's Mac and CI's Linux; nothing here may
	// assume the field arrives with its value already selected.
	await tabTo(page, page.getByLabel('Given name'), 'the Given name field on the edit form');
	await page.keyboard.press('ControlOrMeta+a');
	await page.keyboard.type('Jane');

	await tabTo(page, page.getByRole('button', { name: 'Save' }), 'the Save button on the edit form');
	await page.keyboard.press('Enter');

	// Stage 3 -- the refusal, and the one deliberate override. showModal()
	// traps Tab inside the dialog and lands focus on Cancel, so the
	// confirm button is reachable without ever leaving it.
	const dialog = page.getByRole('dialog', { name: 'Possible duplicate Client' });
	await expect(dialog).toBeVisible();
	await tabTo(
		page,
		dialog.getByRole('button', { name: 'Save as a different person' }),
		'the override button in the duplicate-Client dialog'
	);
	await page.keyboard.press('Enter');

	// Done looks like: the save went through under the new name, and the
	// hub is showing it.
	await expect(page).toHaveURL(new RegExp(`${hubURL}$`));
	await expect(page.getByRole('heading', { level: 1, name: 'Jane Client' })).toBeVisible();

	// Stage 4 -- the Engagement Request, whose only door is this hub.
	await tabTo(
		page,
		page.getByRole('main').getByRole('link', { name: 'Start new work with Jane Client' }),
		'the "start new work" link on the Client detail hub'
	);
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(new RegExp(`${hubURL}/engagement-requests/new$`));

	// The kind is chosen with an arrow key, which is how a radio group is
	// operated -- Tab reaches the group, it does not move within it.
	// Postpartum on purpose: it leaves the due date optional, so what this
	// asserts is the group rather than a date field's segment typing.
	await tabTo(
		page,
		page.getByRole('radio', { name: 'Birth', exact: true }),
		'the "Birth" radio on the Engagement Request form'
	);
	await page.keyboard.press('ArrowDown');
	await expect(page.getByRole('radio', { name: 'Postpartum', exact: true })).toBeChecked();

	// An Owner holds approval authority, so her ask and its approval
	// collapse into one act -- the button says so, and the walk ends back
	// on the hub with a credit spent.
	await tabTo(
		page,
		page.getByRole('button', { name: 'Start work with Jane Client' }),
		'the submit button on the Engagement Request form'
	);
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(new RegExp(`${hubURL}$`));
});

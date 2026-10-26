import { expect, test, type Locator, type Page } from '@playwright/test';
import {
	seedClient,
	seedContractorDoula,
	seedPortalClient,
	PORTAL_CLIENT_PASSWORD
} from './portalClient';
import { seedEngagementRequest } from './stack';
import { STAFF_PASSWORD, uniqueEmail } from './staffSignup';
import { enterPractice } from './auth';

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
async function signInByKeyboard(page: Page, email: string, password: string, practiceId: string) {
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
	await page.keyboard.type(password);

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
	const invited = uniqueEmail('doula');

	// Stage 1 -- sign in by keyboard. seedPortalClient's Owner has no
	// second factor (#1854), so the plain password walk admits her with
	// no challenge, as it does a real Owner straight after signup.
	await signInByKeyboard(page, seeded.staffEmail, STAFF_PASSWORD, practiceId);

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

	await tabTo(page, page.getByLabel('Email address'), 'the email field on the invite form');
	await page.keyboard.type(invited);

	// The form opens with nothing chosen (#1912), so both questions are
	// answered the way a keyboard user answers them: Space on each.
	await tabTo(page, page.getByRole('checkbox', { name: 'Doula' }), 'the Doula role checkbox');
	await page.keyboard.press('Space');
	await tabTo(page, page.getByRole('radio', { name: 'Employee' }), 'the Employee radio');
	await page.keyboard.press('Space');

	await tabTo(page, page.getByRole('button', { name: 'Send invite' }), 'the Send invite button');
	await page.keyboard.press('Enter');

	// Done looks like: the Invitation is out, and the screen says so
	// without anyone having touched a pointing device.
	await expect(
		page.getByText(`An email with a link to join is on its way to ${invited}`)
	).toBeVisible();
});

/*
 * The second walk (#516): three screens under clients/[clientId] (the
 * hub, her name question since #2048, and the Engagement Request form),
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
	request,
	context
}) => {
	const seeded = await seedPortalClient(request, 'Riverside Doulas');
	const { practiceId, clientId, staffHeaders } = seeded;

	// The record the rename below collides with. Plain, no portal account:
	// it exists only to be matched. Family name matches the seeded
	// Client's own ("Client", from seedPortalClient) rather than an
	// unrelated one: #814 narrowed the edit-path collision to an exact
	// given-and-family match, so a rename that only retypes the given
	// name needs the same family name here to still trigger it.
	await seedClient(request, practiceId, staffHeaders, { givenName: 'Jane', familyName: 'Client' });

	// A cookie injection rather than signInByKeyboard: the "Renata" walk
	// above already walks the login form by keyboard, and this walk starts
	// at the hub.
	await enterPractice(context, page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// The one `goto` inside a walk, and it is an entry point rather than a
	// shortcut past a control: #516 asks for the path that starts *at* the
	// detail hub. The roster row that links to the hub is therefore not
	// walked here -- the Clients list is already scanned by axe, and its
	// row link is the same `Link` primitive the first walk proves
	// reachable on the Staff roster.
	const hubURL = `/practices/${practiceId}/clients/${clientId}`;
	await page.goto(hubURL);

	// Stage 1 -- reach the name question from the hub (#2048): the first
	// Change on her record is the given name row's, and it opens the
	// details journey's name question, through the journey's door.
	await tabTo(
		page,
		page.getByRole('main').getByRole('link', { name: 'Change', exact: true }).first(),
		"the given name row's Change on the Client detail hub"
	);
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(new RegExp(String.raw`${hubURL}/details/name\?from=record$`));

	// Stage 2 -- retype the given name. ControlOrMeta so the selection
	// works on both a developer's Mac and CI's Linux; nothing here may
	// assume the field arrives with its value already selected.
	await tabTo(page, page.getByLabel('Given name'), 'the Given name field on the name question');
	await page.keyboard.press('ControlOrMeta+a');
	await page.keyboard.type('Jane');

	await tabTo(page, page.getByRole('button', { name: 'Save' }), 'the Save button on the name question');
	await page.keyboard.press('Enter');

	// Stage 3 -- the refusal, and the one deliberate override. showModal()
	// traps Tab inside the dialog and lands focus on Cancel, so the
	// confirm button is reachable without ever leaving it.
	const dialog = page.getByRole('dialog', { name: 'Possible duplicate Client' });
	await expect(dialog).toBeVisible();
	await tabTo(
		page,
		dialog.getByRole('button', { name: 'Yes, a different person' }),
		'the override button in the duplicate-Client dialog'
	);
	await page.keyboard.press('Enter');

	// Done looks like: the save went through under the new name, and the
	// hub is showing it.
	await expect(page).toHaveURL(new RegExp(`${hubURL}$`));
	await expect(page.getByRole('heading', { level: 1, name: 'Jane Client' })).toBeVisible();
	// #1710: the hub says the save happened, and a keyboard user starts
	// from that message.
	await expect(page.getByText("Jane's details saved.")).toBeFocused();

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
	// collapse into one act -- the button says so, and the walk ends on
	// the Engagement it started, with a credit spent (#1611). The status
	// message there takes focus, so a keyboard user starts from it.
	await tabTo(
		page,
		page.getByRole('button', { name: 'Start work with Jane Client' }),
		'the submit button on the Engagement Request form'
	);
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/engagements/[^/?]+$`));
	await expect(page.getByText('Work with Jane started.')).toBeFocused();
});

/*
 * The third walk (#525, ADR-0017): a contractor Doula's Add a Client
 * door has exactly one control, the explainer's own "Set up a Practice"
 * link -- everything else on the screen the earlier walks already prove
 * reachable belongs to the search form this session never renders.
 */
test('A contractor Doula reaches the explainer\'s "Set up a Practice" link, with no pointer', async ({
	page,
	request
}) => {
	const seeded = await seedPortalClient(request, 'Riverside Doulas');
	const { practiceId } = seeded;
	const contractor = await seedContractorDoula(request, practiceId, seeded.staffHeaders);

	await signInByKeyboard(page, contractor.email, PORTAL_CLIENT_PASSWORD, practiceId);

	// Entry point, same as the Client walk's own hubURL goto above: a
	// contractor Doula does not see "Find or add a Client" on her Clients
	// list at all (#539, ADR-0017) -- her only link to this door is the
	// empty-state explainer on that list, one control this walk does not
	// re-prove.
	await page.goto(`/practices/${practiceId}/clients/search`);
	await expect(page.getByRole('heading', { level: 1, name: 'Add a Client' })).toBeVisible();

	await tabTo(
		page,
		page.getByRole('link', { name: 'Set up a Practice' }),
		'the "Set up a Practice" link on the contractor Doula\'s Add a Client door'
	);
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(/\/signup$/);
});

/*
 * The fourth walk (#528): the Overview hub's door onto a pending
 * Engagement Request, and both decisions the approval screen offers.
 * Approve is only reached and held -- pressing it would decide the one
 * Request this walk seeded, and the walk still has Refuse left to prove.
 * Refuse is the one actually pressed, so the walk demonstrates a real
 * write path end to end and lands where the endpoint's own success
 * behavior says it should: back on the Client's own record.
 */
test('An Owner decides a pending Engagement Request, with no pointer at any step', async ({
	page,
	request,
	context
}) => {
	const seeded = await seedPortalClient(request, 'Riverside Doulas');
	const { practiceId, staffId, staffHeaders } = seeded;

	// A Client of her own, not the seeded practice's existing one -- a
	// live Engagement already on that Client renders this screen's "does
	// not stop this approval" Notice, which is a different, untested-here
	// path (out of scope for this walk).
	const clientId = await seedClient(request, practiceId, staffHeaders, {
		givenName: 'Robin',
		familyName: 'Waiting'
	});
	const requestId = seedEngagementRequest(clientId, practiceId, staffId);

	// A cookie injection rather than signInByKeyboard: the "Renata" walk
	// above already walks the login form by keyboard, and this walk starts
	// at the hub.
	await enterPractice(context, page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// Stage 2 -- the Overview hub's own "Review requests" link is the
	// door onto the pending-Requests inbox.
	await tabTo(
		page,
		page.getByRole('main').getByRole('link', { name: 'Review requests' }),
		'the Review requests link on the Overview hub'
	);
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/engagement-requests$`));

	// Stage 3 -- the inbox row's own link is the door onto the decision;
	// the row links the Client's name, which is her given name alone.
	await tabTo(
		page,
		page.getByRole('main').getByRole('link', { name: 'Robin', exact: true }),
		"Robin's row link on the pending-Requests inbox"
	);
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/engagement-requests/${requestId}$`));

	// Stage 4 -- "Do you approve this work?" (#2043). With neither answer
	// chosen, Tab lands on the group's first radio, Approve; the arrow key
	// moves to Refuse, which reveals the reason under it. Approve is
	// reachable but not chosen: this Request is decided exactly once, and
	// Refuse is the decision this walk proves.
	const approveRadio = page.getByRole('radio', { name: 'Approve and start the work' });
	await tabTo(page, approveRadio, 'the Approve answer on the approval screen');
	await expect(approveRadio).toBeFocused();
	await page.keyboard.press('ArrowDown');
	await expect(page.getByRole('radio', { name: 'Refuse' })).toBeChecked();

	await tabTo(
		page,
		page.getByLabel('Why are you refusing this?'),
		'the refusal-reason field on the approval screen'
	);
	await page.keyboard.type('Not the right fit for this practice right now.');

	await tabTo(
		page,
		page.getByRole('main').getByRole('button', { name: 'Continue' }),
		'the Continue button on the approval screen'
	);
	await page.keyboard.press('Enter');

	// Done looks like: the refusal went through, and the screen is back
	// on the Client's own record -- the endpoint's documented success
	// behavior.
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/clients/${clientId}$`));
});

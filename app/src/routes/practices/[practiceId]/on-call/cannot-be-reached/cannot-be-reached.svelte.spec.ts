import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { cannotBeReachedDraft as draft } from '#lib/cannotBeReachedDraft.svelte.js';
import type { GapPreview } from '#lib/cannotBeReached.js';
import { jsonResponse } from '#lib/testResponse.js';
import { captureLanding } from '#lib/testOutcome.js';
import { practiceSession, toPageState } from '../../../../routeFixture.js';
import { goto, pageState } from '#testing/routePage.js';
import Check from './check/+page.svelte';
import Cover from './cover/+page.svelte';
import Layout from './+layout.svelte';
import Which from './+page.svelte';
import When from './when/+page.svelte';
import { ownerSession, practiceId, preview, roster, seedDraft } from './journeyFixture.js';

vi.mock('$app/state', () => import('#testing/routePage.js'));
vi.mock('$app/navigation', () => import('#testing/routePage.js'));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const base = `/practices/${practiceId}/on-call/cannot-be-reached`;
const doulaSession = practiceSession(['doula'], { staffId: 'staff-2' });

interface Reads {
	preview?: GapPreview | Response;
	roster?: unknown;
}

function answer({ preview: previewAnswer = preview, roster: rosterAnswer = roster }: Reads = {}) {
	apiFetchWithSession.mockImplementation((path: string) => {
		const body = path.includes('/coverage-gaps/preview') ? previewAnswer : rosterAnswer;
		return Promise.resolve(body instanceof Response ? body : jsonResponse(body));
	});
}

function at(step: string, { session = ownerSession, search = '' } = {}) {
	Object.assign(pageState, toPageState({ params: { practiceId }, url: `https://example.test${base}${step}${search}`, pageData: session } as never));
}

beforeEach(() => {
	goto.mockReset();
	goto.mockResolvedValue(undefined);
	apiFetchWithSession.mockReset();
	draft.scope = '';
	draft.clear();
	sessionStorage.clear();
	answer();
});

describe('which Doula', () => {
	it('lists the Doulas who are on a birth and chooses none', async () => {
		at('');
		render(Which);

		await expect.element(testPage.getByRole('heading', { name: 'Which Doula cannot be reached?' })).toBeVisible();
		const select = testPage.getByRole('combobox', { name: 'Which Doula cannot be reached?' });
		await expect.element(select).toHaveValue('');
		await expect.element(testPage.getByRole('option', { name: 'Marguerite Okonkwo-Fitzgerald' })).toBeInTheDocument();
		await expect.element(testPage.getByRole('option', { name: 'Nobody Yet' })).not.toBeInTheDocument();
	});

	it('opens on the Doula the roster was narrowed to', async () => {
		at('', { search: '?doula=staff-3' });
		render(Which);

		await expect.element(testPage.getByRole('combobox', { name: 'Which Doula cannot be reached?' })).toHaveValue('staff-3');
	});

	it('asks for a Doula before going on, then keeps her and goes to the time', async () => {
		at('', { search: '?back=days%3D7' });
		render(Which);
		await expect.element(testPage.getByRole('option', { name: 'Marguerite Okonkwo-Fitzgerald' })).toBeInTheDocument();

		await testPage.getByRole('button', { name: 'Continue' }).click();
		await expect.element(testPage.getByRole('alert').first()).toHaveTextContent('Select the Doula who cannot be reached');
		expect(goto).not.toHaveBeenCalled();

		await testPage.getByRole('combobox', { name: 'Which Doula cannot be reached?' }).selectOptions('staff-3');
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(draft.answers).toMatchObject({ staffId: 'staff-3', staffName: 'Marguerite Okonkwo-Fitzgerald' });
		expect(goto).toHaveBeenCalledWith(`${base}/when?back=days%3D7`);
	});

	it('keeps the answers when the same Doula is chosen again', async () => {
		at('', { search: '?doula=staff-3' });
		draft.start(practiceId);
		draft.update({ staffId: 'staff-3', staffName: 'M', cover: { 'engagement-1': 'x' } });
		render(Which);
		await expect.element(testPage.getByRole('option', { name: 'Marguerite Okonkwo-Fitzgerald' })).toBeInTheDocument();

		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(draft.answers.cover).toEqual({ 'engagement-1': 'x' });
	});

	it('says so when no Doula is on a birth', async () => {
		answer({ roster: { doulas: [] } });
		at('');
		render(Which);

		await expect.element(testPage.getByText('No Doula is on a birth with days on call in the next 92 days.')).toBeVisible();
	});

	it('says a failed read and reads again', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ message: 'Nope' }, 500));
		at('');
		render(Which);

		await expect.element(testPage.getByRole('alert')).toBeVisible();
		await testPage.getByRole('button', { name: 'Try again' }).click();
		await expect.element(testPage.getByRole('combobox', { name: 'Which Doula cannot be reached?' })).toBeVisible();
	});

	it('is not asked of a Doula, who goes on to the time', async () => {
		at('', { session: doulaSession });
		render(Which);

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(`${base}/when`, { replaceState: true }));
	});
});

describe('when', () => {
	beforeEach(() => {
		draft.start(practiceId);
		draft.update({ staffId: 'staff-2', staffName: 'Rowan Petrosyan' });
	});

	it('names the Doula for an Owner and says "you" to a Doula', async () => {
		at('/when');
		const owner = await render(When);
		await expect.element(testPage.getByRole('heading', { name: 'When can Rowan not be reached?' })).toBeVisible();
		owner.unmount();

		at('/when', { session: doulaSession });
		render(When);
		await expect.element(testPage.getByRole('heading', { name: 'When can you not be reached?' })).toBeVisible();
	});

	it('goes back to the Doula question when an Owner has chosen nobody', async () => {
		draft.update({ staffId: '' });
		at('/when', { search: '?back=days%3D7' });
		render(When);

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(`${base}?back=days%3D7`, { replaceState: true }));
	});

	it('refuses an empty time, a time that runs backward and a reason that is too long', async () => {
		at('/when');
		render(When);

		await testPage.getByRole('button', { name: 'Continue' }).click();
		await expect.element(testPage.getByRole('alert').first()).toHaveTextContent('Enter when the gap starts');
		await expect.element(testPage.getByRole('alert').first()).toHaveTextContent('Enter when the gap ends');

		await testPage.getByLabelText('From').fill('2026-11-22T08:00');
		await testPage.getByLabelText('Until').fill('2026-11-21T22:00');
		await testPage.getByLabelText('Reason').fill('x'.repeat(501));
		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expect.element(testPage.getByRole('alert').first()).toHaveTextContent('The gap must end after it starts');
		await expect.element(testPage.getByRole('alert').first()).toHaveTextContent('Keep the reason to 500 characters or fewer.');
		expect(goto).not.toHaveBeenCalled();
	});

	it('keeps what was typed and goes on to who is covering', async () => {
		at('/when', { search: '?back=days%3D7' });
		render(When);

		await testPage.getByLabelText('From').fill('2026-11-21T22:00');
		await testPage.getByLabelText('Until').fill('2026-11-22T08:00');
		await testPage.getByLabelText('Reason').fill("Sister's wedding");
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(draft.answers).toMatchObject({ startsAt: '2026-11-21T22:00', endsAt: '2026-11-22T08:00', reason: "Sister's wedding" });
		expect(goto).toHaveBeenCalledWith(`${base}/cover?back=days%3D7`);
		expect(sessionStorage.getItem(`doula-cloud:cannot-be-reached:${practiceId}`)).toContain('Sister');
	});
});

describe('who is covering', () => {
	beforeEach(() => seedDraft());

	it('asks each birth that has another Doula, says when nobody else is on a birth, and leaves a recorded birth as it is', async () => {
		at('/cover');
		render(Cover);

		await expect.element(testPage.getByRole('group', { name: /Wilhelmina Featherstonehaugh-Ravenscroft, Sat Nov 21, 10:00pm to Sun Nov 22, 8:00am/ })).toBeVisible();
		await expect.element(testPage.getByRole('radio', { name: 'Marguerite Okonkwo-Fitzgerald' }).first()).not.toBeChecked();
		await expect.element(testPage.getByText("Nobody else is on Ana Ruiz's birth, so nobody can cover.")).toBeVisible();
		await expect.element(testPage.getByRole('heading', { name: 'Already recorded' })).toBeVisible();
		await expect.element(testPage.getByText('This birth is left as it is.')).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: "Change it on Kim Tran's birth" }))
			.toHaveAttribute('href', `/practices/${practiceId}/engagements/engagement-3#on-call`);
	});

	it('wants an answer for each birth before going on', async () => {
		at('/cover');
		render(Cover);
		await expect.element(testPage.getByRole('button', { name: 'Continue' })).toBeVisible();

		await testPage.getByRole('button', { name: 'Continue' }).click();
		await expect
			.element(testPage.getByRole('alert').first())
			.toHaveTextContent("Choose who is covering Wilhelmina Featherstonehaugh-Ravenscroft's birth, or Nobody yet");
		expect(goto).not.toHaveBeenCalled();

		await testPage.getByRole('radio', { name: 'Nobody yet' }).click();
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(draft.answers.cover).toEqual({ 'engagement-1': 'nobody' });
		expect(goto).toHaveBeenCalledWith(`${base}/check`);
	});

	it('forgets an answer for a birth no longer asked about', async () => {
		draft.setCover('engagement-gone', 'staff-3');
		at('/cover');
		render(Cover);

		await vi.waitFor(() => expect(draft.answers.cover).toEqual({}));
	});

	it('says the births changed when it is sent back from a refused record', async () => {
		at('/cover', { search: '?changed=true' });
		render(Cover);

		await expect.element(testPage.getByRole('status')).toHaveTextContent('The births on call changed since you checked. Check them again.');
	});

	it('says there is no birth to record on, and offers the time again with no Continue', async () => {
		answer({ preview: { births: [] } });
		at('/cover');
		render(Cover);

		await expect.element(testPage.getByText('You are not on call for any birth then.')).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Change the time' })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Continue' })).not.toBeInTheDocument();
	});

	it('says there is nothing more to record when every birth already holds the gap', async () => {
		answer({ preview: { births: [preview.births[2]!] } });
		at('/cover');
		render(Cover);

		await expect.element(testPage.getByText('There is nothing more to record.')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Continue' })).not.toBeInTheDocument();
	});

	it('says a failed read and reads again', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ message: 'Nope' }, 500));
		at('/cover');
		render(Cover);

		await expect.element(testPage.getByRole('alert')).toBeVisible();
		await testPage.getByRole('button', { name: 'Try again' }).click();
		await expect.element(testPage.getByRole('button', { name: 'Continue' })).toBeVisible();
	});

	it('sends a reader with no time back to the time, and one with no Doula back to the Doula', async () => {
		draft.update({ endsAt: '' });
		at('/cover');
		const first = await render(Cover);
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(base.concat('/when'), { replaceState: true }));
		first.unmount();

		draft.update({ staffId: '' });
		at('/cover');
		render(Cover);
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(base, { replaceState: true }));
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});

	it('is a Doula\'s own question about herself', async () => {
		at('/cover', { session: doulaSession });
		render(Cover);

		await expect.element(testPage.getByRole('button', { name: 'Continue' })).toBeVisible();
		expect(String(apiFetchWithSession.mock.calls[0]![0])).toContain('staffId=staff-2');
	});
});

const created = (id: string, covering?: string) => ({ id, engagementId: id, staffId: 'staff-2', staffName: 'R', startsAt: '', endsAt: '', coveringStaffId: covering });

function recordWith(response: Response) {
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (init?.method === 'POST') return Promise.resolve(response);
		return Promise.resolve(jsonResponse(path.includes('/preview') ? preview : roster));
	});
}


describe('check the time', () => {
	beforeEach(() => {
		seedDraft();
		draft.setCover('engagement-1', 'staff-3');
	});

	it('shows the Doula, the time, the reason and who covers each birth, each with a way back', async () => {
		at('/check');
		render(Check);

		await expect.element(testPage.getByRole('heading', { name: 'Check the time before recording it' })).toBeVisible();
		await expect.element(testPage.getByText('Marguerite Okonkwo-Fitzgerald covering')).toBeVisible();
		await expect.element(testPage.getByText('Nobody covering')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Record it on 2 births' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: /Change/ }).first()).toHaveAttribute('href', base);
	});

	it('shows "Not given" for no reason, and leaves out the Doula row for a Doula', async () => {
		draft.update({ reason: '' });
		at('/check', { session: doulaSession });
		render(Check);

		await expect.element(testPage.getByText('Not given')).toBeVisible();
		await expect.element(testPage.getByText('Doula', { exact: true })).not.toBeInTheDocument();
	});

	it('records on every birth, clears the draft, and lands on the roster saying so', async () => {
		recordWith(jsonResponse({ gaps: [created('a', 'staff-3'), created('b')] }, 201));
		const landing = captureLanding(goto);
		at('/check', { search: '?back=days%3D7' });
		render(Check);

		await testPage.getByRole('button', { name: 'Record it on 2 births' }).click();

		await vi.waitFor(() => expect(landing.href).toBe(`/practices/${practiceId}/on-call?days=7`));
		expect(landing.message).toBe(
			'The time Persephone cannot be reached is recorded on 2 births. Every Owner and Admin is told of the 1 with nobody covering.'
		);
		const post = apiFetchWithSession.mock.calls.find(([, init]) => init?.method === 'POST')!;
		expect(JSON.parse(post[1].body as string)).toMatchObject({
			staffId: 'staff-2',
			reason: expect.stringContaining('https://'),
			births: [
				{ engagementId: 'engagement-1', coveringStaffId: 'staff-3' },
				{ engagementId: 'engagement-2' }
			]
		});
		expect(draft.answers.startsAt).toBe('');
	});

	it('goes back to who is covering when the births changed since they were checked', async () => {
		recordWith(jsonResponse({ message: 'The births on call changed since you checked. Check them again.' }, 409));
		at('/check');
		render(Check);

		await testPage.getByRole('button', { name: 'Record it on 2 births' }).click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(`${base}/cover?changed=true`));
	});

	it('says any other refusal on the page and keeps the answers', async () => {
		recordWith(jsonResponse({ message: 'The coverage gap could not be saved.' }, 400));
		at('/check');
		render(Check);

		await testPage.getByRole('button', { name: 'Record it on 2 births' }).click();

		await expect.element(testPage.getByRole('alert').first()).toHaveTextContent('The coverage gap could not be saved.');
		expect(draft.answers.startsAt).not.toBe('');
	});

	it('goes back to who is covering when a birth has no answer yet', async () => {
		draft.update({ cover: {} });
		at('/check');
		render(Check);

		await testPage.getByRole('button', { name: 'Record it on 2 births' }).click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(`${base}/cover?changed=true`));
	});

	it('says a failed read and reads again', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ message: 'Nope' }, 500));
		at('/check');
		render(Check);

		await expect.element(testPage.getByRole('alert')).toBeVisible();
		await testPage.getByRole('button', { name: 'Try again' }).click();
		await expect.element(testPage.getByRole('button', { name: 'Record it on 2 births' })).toBeVisible();
	});

	it('sends a reader with nothing answered back to the start of the act', async () => {
		draft.clear();
		at('/check');
		render(Check);

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(base, { replaceState: true }));
		draft.update({ staffId: 'staff-2' });
		at('/check');
		render(Check);
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(`${base}/when`, { replaceState: true }));
	});

	it('records for a Doula in the first person', async () => {
		recordWith(jsonResponse({ gaps: [created('a', 'staff-3')] }, 201));
		const landing = captureLanding(goto);
		at('/check', { session: doulaSession });
		render(Check);

		await testPage.getByRole('button', { name: 'Record it on 2 births' }).click();

		await vi.waitFor(() => expect(landing.message).toBe('The time you cannot be reached is recorded on 1 birth.'));
	});
});

describe('the layout', () => {
	it('opens the draft for the Practice and draws its page', async () => {
		at('');
		draft.scope = '';
		render(Layout, { children: (() => {}) as never });

		expect(draft.scope).toBe(practiceId);
	});
});

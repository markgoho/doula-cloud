import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import BirthOutcomeSection from './BirthOutcomeSection.svelte';
import type { BirthOutcomeResult } from '#lib/engagementDetail.js';

const recorded: BirthOutcomeResult = {
	kind: 'recorded',
	facts: { birthOutcome: 'live_birth', pregnancyEndedOn: '2026-08-14' }
};

/*
 * The map the component hands `onRecord` alongside every request (#488):
 * the BFF's own field names against this component's control ids, so a
 * `details` entry the endpoint sends lands on the right box. Asserted on
 * every call rather than ignored with `expect.anything()`, because a
 * key that stopped matching would silently untarget the refusal.
 */
const BIRTH_OUTCOME_FIELD_IDS = {
	birthOutcome: 'birth-outcome-live_birth',
	pregnancyEndedOn: 'pregnancy-ended-on-day'
};

const frozen: BirthOutcomeResult = {
	kind: 'confirmable',
	message: 'this Engagement already has a birth outcome; only a Practice Owner can correct it'
};

interface SetupOptions {
	outcome?: string;
	endedOn?: string;
	canRecord?: boolean;
	canCorrect?: boolean;
	result?: BirthOutcomeResult;
	/** What the *second* send answers, for the press-through: the first
	 * is refused as frozen and the re-send with `correction: true` is a
	 * separate answer. */
	thenResult?: BirthOutcomeResult;
}

async function setup({
	outcome,
	endedOn,
	canRecord = true,
	canCorrect = true,
	result = recorded,
	thenResult
}: SetupOptions = {}) {
	let isFirst = true;
	const onRecord = vi.fn(async () => {
		if (isFirst || thenResult === undefined) {
			isFirst = false;
			return result;
		}
		return thenResult;
	});
	await render(BirthOutcomeSection, { outcome, endedOn, canRecord, canCorrect, onRecord });
	return { onRecord };
}

/** Opens the question, which is deliberately behind a control rather than
 * on screen from the start. */
async function openForm(label = 'Record what happened') {
	await page.getByRole('button', { name: label }).click();
}

async function chooseAndDate(option: string, month: string, day: string, year: string) {
	await page.getByLabelText(option).click();
	await page.getByLabelText('Month').fill(month);
	await page.getByLabelText('Day').fill(day);
	await page.getByLabelText('Year').fill(year);
}

describe('BirthOutcomeSection', () => {
	it('says nothing is recorded, and does not ask the question unprompted', async () => {
		await setup();

		await expect.element(page.getByText('Nothing recorded yet.', { exact: false })).toBeVisible();
		await expect
			.element(page.getByText('What happened to the pregnancy?'))
			.not.toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Record what happened' })).toBeVisible();
	});

	it('offers a contractor Doula no control at all', async () => {
		await setup({ canRecord: false, canCorrect: false });

		await expect.element(page.getByText('Nothing recorded yet.', { exact: false })).toBeVisible();
		await expect
			.element(page.getByRole('button', { name: 'Record what happened' }))
			.not.toBeInTheDocument();
	});

	it('asks about the pregnancy without presuming a living baby', async () => {
		await setup();
		await openForm();

		await expect.element(page.getByText('What happened to the pregnancy?')).toBeVisible();
		await expect.element(page.getByLabelText('The baby was born alive')).toBeVisible();
		await expect
			.element(page.getByLabelText('The pregnancy ended without a living baby'))
			.toBeVisible();
		await expect
			.element(page.getByLabelText('The Practice never learned what happened'))
			.toBeVisible();
	});

	it('asks for the date the pregnancy ended, as three boxes', async () => {
		await setup();
		await openForm();
		await page.getByLabelText('The baby was born alive').click();

		await expect.element(page.getByText('When did the pregnancy end?')).toBeVisible();
		await expect.element(page.getByLabelText('Month')).toBeVisible();
		await expect.element(page.getByLabelText('Day')).toBeVisible();
		await expect.element(page.getByLabelText('Year')).toBeVisible();
	});

	it('does not ask for a date at all when the Practice never learned', async () => {
		await setup();
		await openForm();
		await page.getByLabelText('The Practice never learned what happened').click();

		await expect.element(page.getByText('When did the pregnancy end?')).not.toBeInTheDocument();
		await expect.element(page.getByLabelText('Month')).not.toBeInTheDocument();
	});

	it('records the outcome and the date the pregnancy ended', async () => {
		const { onRecord } = await setup();
		await openForm();
		await chooseAndDate('The pregnancy ended without a living baby', '8', '14', '2026');
		await page.getByRole('button', { name: 'Record this outcome' }).click();

		expect(onRecord).toHaveBeenCalledWith(
			{ birthOutcome: 'loss', pregnancyEndedOn: '2026-08-14' },
			BIRTH_OUTCOME_FIELD_IDS
		);
	});

	it('sends no date, and no correction flag, for a first unknown outcome', async () => {
		const { onRecord } = await setup();
		await openForm();
		await page.getByLabelText('The Practice never learned what happened').click();
		await page.getByRole('button', { name: 'Record this outcome' }).click();

		expect(onRecord).toHaveBeenCalledWith(
			{ birthOutcome: 'unknown', pregnancyEndedOn: undefined },
			BIRTH_OUTCOME_FIELD_IDS
		);
	});

	it('refuses a submit with nothing chosen', async () => {
		const { onRecord } = await setup();
		await openForm();
		await page.getByRole('button', { name: 'Record this outcome' }).click();

		await expect
			.element(page.getByText('Select what happened to the pregnancy').first())
			.toBeVisible();
		expect(onRecord).not.toHaveBeenCalled();
	});

	it('refuses an outcome with no date, naming the date', async () => {
		const { onRecord } = await setup();
		await openForm();
		await page.getByLabelText('The baby was born alive').click();
		await page.getByRole('button', { name: 'Record this outcome' }).click();

		await expect
			.element(page.getByText('Enter the date the pregnancy ended').first())
			.toBeVisible();
		expect(onRecord).not.toHaveBeenCalled();
	});

	it('refuses a date that is not a real one', async () => {
		const { onRecord } = await setup();
		await openForm();
		await chooseAndDate('The baby was born alive', '2', '30', '2026');
		await page.getByRole('button', { name: 'Record this outcome' }).click();

		await expect
			.element(page.getByText('The date the pregnancy ended must be a real date').first())
			.toBeVisible();
		expect(onRecord).not.toHaveBeenCalled();
	});

	it('reads a recorded outcome back, in words', async () => {
		await setup({ outcome: 'live_birth', endedOn: '2026-08-14', canCorrect: false });

		await expect.element(page.getByText('What happened')).toBeVisible();
		await expect.element(page.getByText('The baby was born alive')).toBeVisible();
		await expect.element(page.getByText('Date the pregnancy ended', { exact: true })).toBeVisible();
	});

	it('offers no edit affordance on a recorded outcome to anyone but an Owner', async () => {
		await setup({ outcome: 'loss', endedOn: '2026-08-14', canCorrect: false });

		await expect
			.element(page.getByRole('button', { name: 'Correct what was recorded' }))
			.not.toBeInTheDocument();
		await expect
			.element(page.getByRole('button', { name: 'Remove this record' }))
			.not.toBeInTheDocument();
	});

	it('lets an Owner correct a recorded outcome, naming it as a correction', async () => {
		const { onRecord } = await setup({ outcome: 'loss', endedOn: '2026-08-14' });
		await openForm('Correct what was recorded');

		// Prefilled with what is recorded, so a correction of the date alone
		// does not mean retyping the outcome.
		await expect.element(page.getByLabelText('Year')).toHaveValue('2026');
		await page.getByLabelText('The baby was born alive').click();
		await page.getByRole('button', { name: 'Save the correction' }).click();

		expect(onRecord).toHaveBeenCalledWith(
			{ birthOutcome: 'live_birth', pregnancyEndedOn: '2026-08-14', correction: true },
			BIRTH_OUTCOME_FIELD_IDS
		);
	});

	it('lets an Owner clear a wrongly recorded outcome, behind a named confirmation', async () => {
		const { onRecord } = await setup({ outcome: 'loss', endedOn: '2026-08-14' });
		await page.getByRole('button', { name: 'Remove this record' }).click();

		await expect
			.element(page.getByRole('heading', { name: 'Remove the recorded outcome?' }))
			.toBeVisible();
		await page.getByRole('dialog').getByRole('button', { name: 'Remove this record' }).click();

		expect(onRecord).toHaveBeenCalledWith(
			// eslint-disable-next-line unicorn/no-null -- the wire value for a clear.
			{ birthOutcome: null, correction: true },
			BIRTH_OUTCOME_FIELD_IDS
		);
	});

	it('renders a frozen row as a confirmation an Owner presses through', async () => {
		const { onRecord } = await setup({ result: frozen });
		await openForm();
		await chooseAndDate('The baby was born alive', '8', '14', '2026');
		await page.getByRole('button', { name: 'Record this outcome' }).click();

		await expect
			.element(page.getByRole('heading', { name: 'This Engagement already has a recorded outcome' }))
			.toBeVisible();
		await expect.element(page.getByText(frozen.message ?? '')).toBeVisible();

		await page.getByRole('button', { name: 'Overwrite it with what I entered' }).click();

		expect(onRecord).toHaveBeenLastCalledWith(
			{ birthOutcome: 'live_birth', pregnancyEndedOn: '2026-08-14', correction: true },
			BIRTH_OUTCOME_FIELD_IDS
		);
	});

	it('reports a refusal of the pressed-through correction itself', async () => {
		await setup({
			result: frozen,
			thenResult: {
				kind: 'errors',
				errors: [{ message: 'only a Practice Owner can correct a recorded birth outcome' }]
			}
		});
		await openForm();
		await chooseAndDate('The baby was born alive', '8', '14', '2026');
		await page.getByRole('button', { name: 'Record this outcome' }).click();
		await page.getByRole('button', { name: 'Overwrite it with what I entered' }).click();

		await expect
			.element(page.getByText('only a Practice Owner can correct a recorded birth outcome').first())
			.toBeVisible();
	});

	it('renders a frozen row as itself to a reader who cannot press through it', async () => {
		await setup({ canCorrect: false, result: frozen });
		await openForm();
		await chooseAndDate('The baby was born alive', '8', '14', '2026');
		await page.getByRole('button', { name: 'Record this outcome' }).click();

		await expect
			.element(page.getByRole('heading', { name: 'This Engagement already has a recorded outcome' }))
			.not.toBeInTheDocument();
		await expect.element(page.getByText(frozen.message ?? '').first()).toBeVisible();
	});

	it('renders every other refusal as itself', async () => {
		await setup({
			result: { kind: 'errors', errors: [{ message: 'This Engagement has no birth outcome to correct' }] }
		});
		await openForm();
		await page.getByLabelText('The Practice never learned what happened').click();
		await page.getByRole('button', { name: 'Record this outcome' }).click();

		await expect
			.element(page.getByText('This Engagement has no birth outcome to correct').first())
			.toBeVisible();
	});

	it('reports a refused clear where it happened, not as a refused form', async () => {
		await setup({
			outcome: 'loss',
			endedOn: '2026-08-14',
			result: { kind: 'errors', errors: [{ message: 'This Engagement has no birth outcome to correct' }] }
		});
		await page.getByRole('button', { name: 'Remove this record' }).click();
		await page.getByRole('dialog').getByRole('button', { name: 'Remove this record' }).click();

		await expect
			.element(page.getByText('This Engagement has no birth outcome to correct').first())
			.toBeVisible();
		// A Notice, not the summary: no field was filled in, so there is
		// nothing to send her back to (#467).
		await expect.element(page.getByText('There is a problem')).not.toBeInTheDocument();
	});

	// A dropped connection is not a refusal the BFF wrote; without the
	// wrapper it would reject into nothing and leave the screen unchanged.
	it('says so when the clear never reaches the service', async () => {
		const onRecord = vi.fn(async () => {
			throw new Error('Failed to fetch');
		});
		await render(BirthOutcomeSection, {
			outcome: 'loss',
			endedOn: '2026-08-14',
			canRecord: true,
			canCorrect: true,
			onRecord
		});
		await page.getByRole('button', { name: 'Remove this record' }).click();
		await page.getByRole('dialog').getByRole('button', { name: 'Remove this record' }).click();

		await expect.element(page.getByText('Failed to fetch')).toBeVisible();
	});

	// The date refusal lives beside the summary's own copy, so it has to
	// be cleared beside it too.
	it('reopens the question with no stale refusal against the date boxes', async () => {
		await setup();
		await openForm();
		await page.getByLabelText('The baby was born alive').click();
		await page.getByRole('button', { name: 'Record this outcome' }).click();
		await expect
			.element(page.getByText('Enter the date the pregnancy ended').first())
			.toBeVisible();

		await page.getByRole('button', { name: 'Cancel' }).click();
		await openForm();
		await page.getByLabelText('The baby was born alive').click();

		await expect
			.element(page.getByText('Enter the date the pregnancy ended'))
			.not.toBeInTheDocument();
	});

	it('closes the question on Cancel, asking nothing', async () => {
		const { onRecord } = await setup();
		await openForm();
		await page.getByRole('button', { name: 'Cancel' }).click();

		await expect
			.element(page.getByText('What happened to the pregnancy?'))
			.not.toBeInTheDocument();
		expect(onRecord).not.toHaveBeenCalled();
	});
});

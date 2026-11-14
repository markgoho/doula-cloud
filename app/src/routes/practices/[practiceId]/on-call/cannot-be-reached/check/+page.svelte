<script lang="ts">
	/*
	 * The end of recording a time a Doula cannot be reached (#2934, GOV.UK's
	 * Check answers pattern): the time and, for each birth, who is covering,
	 * with a way back to each, and the one write. The births are asked of the
	 * BFF again, so what is shown is what a press records; if they changed
	 * since step 3 the write is refused and step 3 opens on the new set.
	 */
	import { goto } from '$app/navigation';
	import { apiFetchWithSession } from '#lib/api.js';
	import { cannotBeReachedDraft as draft } from '#lib/cannotBeReachedDraft.svelte.js';
	import {
		coverAnswer,
		coverFor,
		firstName,
		isWritable,
		previewGaps,
		recordGaps,
		recordButton,
		recordedMessage,
		type GapPreview,
		type PreviewBirth
	} from '#lib/cannotBeReached.js';
	import { formatGapTimes } from '#lib/coverageGap.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import ReadFailure from '#lib/components/molecules/ReadFailure.svelte';
	import CheckAnswers, { type AnswerSection } from '#lib/components/templates/CheckAnswers.svelte';
	import { FormSubmission, orThrownMessage } from '#lib/formSubmission.svelte.js';
	import { errorsFromCause, RefusalError } from '#lib/formErrors.js';
	import { ON_CALL_READ } from '#lib/onCall.js';
	import { gotoWithOutcome } from '#lib/outcome.js';
	import { instantFromWall } from '#lib/zonedTime.js';
	import { JOURNEY_NAME, Journey } from '../journey.svelte.js';

	const journey = new Journey();
	const now = new Date();
	const submission = new FormSubmission();

	let preview = $state.raw<GapPreview>();
	let failure = $state('');

	const timeZone = $derived(journey.session.timezone);
	const writable = $derived((preview?.births ?? []).filter(isWritable));
	const startsAt = $derived(instantFromWall(draft.answers.startsAt, timeZone));
	const endsAt = $derived(instantFromWall(draft.answers.endsAt, timeZone));
	const coveringName = (birth: PreviewBirth) =>
		birth.coverOptions.find((option) => option.staffId === coverFor(birth, draft.answers.cover))?.name;

	$effect(() => {
		const { staffId, startsAt: from, endsAt: until } = draft.answers;
		if (!staffId && journey.isOwnerView) void goto(journey.href('doula'), { replaceState: true });
		else if (!from || !until) void goto(journey.href('when'), { replaceState: true });
	});

	async function read() {
		failure = '';
		preview = undefined;
		if (!journey.staffId || Number.isNaN(startsAt.getTime()) || Number.isNaN(endsAt.getTime())) return;
		try {
			preview = await previewGaps(apiFetchWithSession, journey.practiceId, journey.staffId, startsAt, endsAt);
		} catch (error) {
			failure = errorsFromCause(error)[0]!.message;
		}
	}
	void read();

	const sections = $derived.by<AnswerSection[]>(() => {
		if (Number.isNaN(startsAt.getTime()) || Number.isNaN(endsAt.getTime())) return [];
		const answers = [];
		if (journey.isOwnerView) {
			answers.push({ label: 'Doula', value: draft.answers.staffName, changeHref: journey.href('doula'), changes: 'Doula' });
		}
		answers.push(
			{
				label: 'When',
				value: formatGapTimes(startsAt.toISOString(), endsAt.toISOString(), timeZone, now),
				changeHref: journey.href('when'),
				changes: 'time'
			},
			{ label: 'Reason', value: draft.answers.reason.trim() || 'Not given', changeHref: journey.href('when'), changes: 'reason' }
		);
		for (const birth of writable) {
			answers.push({
				label: birth.clientName,
				value: coverAnswer(coveringName(birth)),
				changeHref: journey.href('cover'),
				changes: `who is covering ${birth.clientName}'s birth`
			});
		}
		return [{ answers }];
	});

	async function record() {
		await submission.run(async () => {
			const chosen = writable.map((birth) => ({ birth, cover: coverFor(birth, draft.answers.cover) }));
			if (chosen.length === 0 || chosen.some((entry) => entry.cover === undefined)) {
				await goto(journey.href('cover', { changed: 'true' }));
				return;
			}
			try {
				const gaps = await recordGaps(apiFetchWithSession, journey.practiceId, {
					staffId: journey.staffId,
					startsAt: startsAt.toISOString(),
					endsAt: endsAt.toISOString(),
					reason: draft.answers.reason.trim() || undefined,
					births: chosen.map((entry) => ({ engagementId: entry.birth.engagementId, coveringStaffId: entry.cover || undefined }))
				});
				const subject = journey.isOwnerView ? firstName(draft.answers.staffName) : 'you';
				const message = recordedMessage(subject, gaps.length, gaps.filter((gap) => !gap.coveringStaffId).length);
				await gotoWithOutcome(journey.rosterHref, message, [ON_CALL_READ]);
				draft.clear();
			} catch (error) {
				if (error instanceof RefusalError && error.status === 409) {
					await goto(journey.href('cover', { changed: 'true' }));
					return;
				}
				throw error;
			}
		}, orThrownMessage);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#if failure}
	<ReadFailure message={failure} onRetry={read} />
{:else if !preview}
	<Skeleton variant="row" lines={4} label="Loading the births" />
{:else}
	<CheckAnswers
		journey={JOURNEY_NAME}
		steps={journey.steps('check').map((step) => ({ ...step, status: 'completed' }))}
		backHref={journey.backFrom('check')}
		title="Check the time before recording it"
		{sections}
		errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	>
		{#snippet actions()}
			<Button label={recordButton(writable.length)} loading={submission.isSubmitting} onClick={record} />
		{/snippet}
	</CheckAnswers>
{/if}

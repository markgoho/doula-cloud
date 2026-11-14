<script lang="ts">
	/*
	 * Step 3 of recording a time a Doula cannot be reached (#2934): who is
	 * covering each birth the time meets. Only a Doula on the same birth can
	 * cover it, so the options are asked of the BFF after the time is known,
	 * and nothing is chosen when a birth's question opens. A birth that
	 * already holds a gap of hers over the time is left as it is and said so.
	 */
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { untrack } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { cannotBeReachedDraft as draft } from '#lib/cannotBeReachedDraft.svelte.js';
	import {
		NOBODY_YET,
		NOT_ON_CALL_THEN,
		NOTHING_TO_RECORD,
		birthLegend,
		coverFor,
		coverRequired,
		isAsked,
		isWritable,
		nobodyElse,
		previewGaps,
		BIRTHS_CHANGED,
		type GapPreview
	} from '#lib/cannotBeReached.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import ErrorSummary, { type FormError } from '#lib/components/molecules/ErrorSummary.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
	import ReadFailure from '#lib/components/molecules/ReadFailure.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { errorsFromCause } from '#lib/formErrors.js';
	import { instantFromWall } from '#lib/zonedTime.js';
	import { JOURNEY_NAME, Journey } from '../journey.svelte.js';

	const journey = new Journey();
	const now = new Date();
	const hasChanged = untrack(() => page.url.searchParams.has('changed'));

	let preview = $state.raw<GapPreview>();
	let failure = $state('');
	let errors = $state<FormError[]>([]);

	const timeZone = $derived(journey.session.timezone);
	const births = $derived(preview?.births ?? []);
	const writable = $derived(births.filter(isWritable));
	const recorded = $derived(births.filter((birth) => !isWritable(birth)));
	const nameId = (engagementId: string) => `cover-${engagementId}`;

	$effect(() => {
		const { staffId, startsAt, endsAt } = draft.answers;
		if (!staffId && journey.isOwnerView) void goto(journey.href('doula'), { replaceState: true });
		else if (!startsAt || !endsAt) void goto(journey.href('when'), { replaceState: true });
	});

	async function read() {
		failure = '';
		preview = undefined;
		const startsAt = instantFromWall(draft.answers.startsAt, timeZone);
		const endsAt = instantFromWall(draft.answers.endsAt, timeZone);
		if (!journey.staffId || Number.isNaN(startsAt.getTime()) || Number.isNaN(endsAt.getTime())) return;
		try {
			preview = await previewGaps(apiFetchWithSession, journey.practiceId, journey.staffId, startsAt, endsAt);
			draft.keepCover(preview.births.filter(isAsked).map((birth) => birth.engagementId));
		} catch (error) {
			failure = errorsFromCause(error)[0]!.message;
		}
	}
	void read();

	function next(event: SubmitEvent) {
		event.preventDefault();
		const missing = writable.filter((birth) => coverFor(birth, draft.answers.cover) === undefined);
		errors = missing.map((birth) => ({
			message: coverRequired(birth.clientName),
			targetId: radioFieldId(nameId(birth.engagementId), birth.coverOptions[0]!.staffId)
		}));
		if (errors.length === 0) void goto(journey.nextAfter('cover'));
	}

	const errorOf = (engagementId: string) => errors.find((entry) => entry.targetId?.startsWith(nameId(engagementId)))?.message;
	const engagementHref = (engagementId: string) =>
		`${resolve('/practices/[practiceId]/engagements/[engagementId]', { practiceId: journey.practiceId, engagementId })}#on-call`;
</script>

{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs, not to arrange anything. -->
<form onsubmit={next} novalidate>
	<QuestionPage
		journey={JOURNEY_NAME}
		steps={journey.steps('cover')}
		backHref={journey.backFrom('cover')}
		question={{ as: 'legend', text: 'Who is covering?' }}
		hint="Only a Doula on the same birth can cover it. Every Owner and Admin is told of each birth left with nobody covering."
		errorSummary={errors.length > 0 ? errorSummary : undefined}
	>
		{#snippet content()}
			{#if hasChanged}
				<Notice variant="info" message={BIRTHS_CHANGED} />
			{/if}
			{#if failure}
				<ReadFailure message={failure} onRetry={read} />
			{:else if !preview}
				<Skeleton variant="row" lines={2} label="Loading the births" />
			{:else}
				{#each writable as birth (birth.engagementId)}
					{#if isAsked(birth)}
						<RadioGroup
							legend={birthLegend(birth, timeZone, now)}
							name={nameId(birth.engagementId)}
							options={[
								...birth.coverOptions.map((option) => ({ value: option.staffId, label: option.name })),
								{ value: NOBODY_YET, label: 'Nobody yet' }
							]}
							value={draft.answers.cover[birth.engagementId]}
							onChange={(value) => draft.setCover(birth.engagementId, value)}
							error={errorOf(birth.engagementId)}
						/>
					{:else}
						<div>
							<Text text={birthLegend(birth, timeZone, now)} step="label" />
							<Text text={nobodyElse(birth.clientName)} tone="variant" />
						</div>
					{/if}
				{/each}
				{#if births.length === 0}
					<Text text={NOT_ON_CALL_THEN} />
					<Link href={journey.href('when')} label="Change the time" />
				{:else if writable.length === 0}
					<Text text={NOTHING_TO_RECORD} />
				{/if}
				{#if recorded.length > 0}
					<section>
						<Heading level={2} text="Already recorded" />
						{#each recorded as birth (birth.engagementId)}
							<div>
								<Text text={birthLegend(birth.existingGap ? { ...birth, startsAt: birth.existingGap.startsAt, endsAt: birth.existingGap.endsAt } : birth, timeZone, now)} step="label" />
								<Text text="This birth is left as it is." tone="variant" />
								<Link href={engagementHref(birth.engagementId)} label={`Change it on ${birth.clientName}'s birth`} />
							</div>
						{/each}
					</section>
				{/if}
			{/if}
		{/snippet}
		{#snippet actions()}
			{#if writable.length > 0}
				<Button type="submit" label="Continue" />
			{/if}
		{/snippet}
	</QuestionPage>
</form>

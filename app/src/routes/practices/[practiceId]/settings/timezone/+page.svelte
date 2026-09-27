<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadPracticeTimezone, savePracticeTimezone } from '#lib/practiceTimezone.js';
	import { TIMEZONE_HINT, TIMEZONE_NEEDED, timezoneOptions } from '#lib/timezones.js';
	import { isOwnerOrAdmin } from '#lib/roles.js';
	import type { PracticeSession } from '../../+layout.js';
	import TimezoneField from '#lib/components/molecules/TimezoneField.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import { errorsFromCause } from '#lib/formErrors.js';
	import { FormSubmission, type FormError } from '#lib/formSubmission.svelte.js';

	const timezoneId = 'practice-timezone';

	/* Every Staff can read the zone -- #1280 widened the GET so
	   `InvoiceSection` could -- but only an Owner or Admin can state it
	   (#1441). Anyone else is shown the zone as a fact, never a Save
	   button that exists only to be refused. */
	const session = $derived((page.data as { session: PracticeSession }).session);
	let canChangeTimezone = $derived(isOwnerOrAdmin(session));

	// docs/api-design.md section 7's Details is keyed by the DTO's own
	// JSON field name -- PUT /api/practices/{practiceId}/timezone's body
	// has exactly one -- so a refusal the BFF wrote for this field lands
	// on this control and in the summary's link to it.
	const timezoneFieldIds = { timezone: timezoneId };

	let timezone = $state('');
	let isSaved = $state(false);
	/* Until the read answers, the zone is unknown: an empty select or an
	   empty read-only row would say "no zone" when the truth is "not yet". */
	let isLoaded = $state(false);
	const submission = new FormSubmission();

	/* The label a person would say, from the same list the select offers,
	   so the read-only zone and the editable one read alike. */
	let timezoneLabel = $derived(
		timezoneOptions(timezone).find((zone) => zone.value === timezone)?.label ?? timezone
	);

	/* The screen's own refusal is already the list `ErrorSummary` wants,
	   so it passes straight through; anything thrown is a refusal the BFF
	   wrote, read for the field it named. Same split the signup page's
	   `mapSignupRefusal` makes. */
	function mapRefusal(refusal: unknown): FormError[] {
		return Array.isArray(refusal) ? refusal : errorsFromCause(refusal, timezoneFieldIds);
	}

	async function load() {
		try {
			const current = await loadPracticeTimezone(apiFetchWithSession, page.params.practiceId!);
			timezone = current.timezone;
		} catch (error) {
			submission.errors = errorsFromCause(error);
		} finally {
			isLoaded = true;
		}
	}

	async function save() {
		isSaved = false;
		await submission.run(async () => {
			if (timezone === '') {
				return [{ message: TIMEZONE_NEEDED, targetId: timezoneId }];
			}
			const saved = await savePracticeTimezone(
				apiFetchWithSession,
				page.params.practiceId!,
				timezone
			);
			timezone = saved.timezone;
			isSaved = true;
		}, mapRefusal);
	}

	onMount(() => {
		void load();
	});
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet readOnlyFields()}
	<DescriptionList items={[{ label: 'Timezone', value: timezoneLabel }]} />
	<Text text={TIMEZONE_HINT} />
	<!-- The payments screen's shape for a fact she can see but not change
	     (#267): a plain sentence naming who holds it, not the BFF's 403. -->
	<Notice variant="status" message="Only a Practice Owner or Admin can change the timezone." />
{/snippet}

{#snippet fields()}
	{#if isSaved}
		<Text text="Saved." />
	{/if}
	<TimezoneField
		id={timezoneId}
		bind:value={timezone}
		error={submission.errorFor(timezoneId)}
		hint={TIMEZONE_HINT}
	/>
	<!--
		GOV.UK's rule for a consequence a person can still act on: it is
		stated before the press, not reported after it. Changing the zone
		is retroactive by construction -- CONTEXT.md's Visit entry derives
		a Visit's type on every read, so nothing is stored to migrate and
		every Visit already recorded is re-judged against the new day the
		moment this is saved. No instant moves; what moves is which
		calendar day an evening Visit belongs to.
	-->
	<WarningText
		message="Changing this changes how Visits already recorded are typed. A Visit worked late in the evening can move between birth and postpartum work, because the day it falls on is worked out in this timezone."
	/>
{/snippet}

{#snippet actions()}
	{#if canChangeTimezone}
		<Button label="Save" loading={submission.isSubmitting} onClick={save} />
	{/if}
{/snippet}

<FormPage
	title="Timezone"
	fieldsets={[{ content: canChangeTimezone ? fields : readOnlyFields }]}
	{actions}
	loading={isLoaded ? undefined : 'Loading the timezone'}
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
/>

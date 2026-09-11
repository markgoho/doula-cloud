<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadPracticeTimezone, savePracticeTimezone } from '#lib/practiceTimezone.js';
	import { TIMEZONE_HINT, TIMEZONE_NEEDED } from '#lib/timezones.js';
	import TimezoneField from '#lib/components/molecules/TimezoneField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import { errorsFromCause } from '#lib/formErrors.js';
	import { FormSubmission, type FormError } from '#lib/formSubmission.svelte.js';

	const timezoneId = 'practice-timezone';

	// docs/api-design.md section 7's Details is keyed by the DTO's own
	// JSON field name -- PUT /api/practices/{practiceId}/timezone's body
	// has exactly one -- so a refusal the BFF wrote for this field lands
	// on this control and in the summary's link to it.
	const timezoneFieldIds = { timezone: timezoneId };

	let timezone = $state('');
	let isSaved = $state(false);
	const submission = new FormSubmission();

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
	<Button label="Save" loading={submission.isSubmitting} onClick={save} />
{/snippet}

<FormPage
	title="Timezone"
	fieldsets={[{ content: fields }]}
	{actions}
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
/>

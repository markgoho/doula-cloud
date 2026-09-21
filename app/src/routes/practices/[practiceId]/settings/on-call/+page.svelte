<script lang="ts">
	/*
	 * When on call starts, and how long it runs (#1093). The Practice's
	 * own rule, stated once by an Owner or an Admin and overridable on a
	 * single birth from that birth's own page.
	 *
	 * Built on `FormPage` + `RadioGroup` + `LabeledField`, the same frame
	 * the Timezone screen uses for the sibling Practice-level setting. The
	 * consequence is stated before the press rather than reported after
	 * it (GOV.UK): nothing stores a window, so saving this moves every
	 * window the Practice has at once.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		loadOnCallSettings,
		saveOnCallSettings,
		type OnCallSettings
	} from '#lib/onCall.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import { errorsFromCause } from '#lib/formErrors.js';
	import { FormSubmission, type FormError } from '#lib/formSubmission.svelte.js';

	const startRuleId = 'on-call-start-rule';
	const startWeekId = 'on-call-start-week';
	const graceDaysId = 'on-call-grace-days';

	// docs/api-design.md section 7's Details is keyed by the DTO's own
	// JSON field name, so a refusal the BFF wrote lands on the control it
	// names and in the summary's link to it.
	const fieldIds = {
		startRule: startRuleId,
		startWeek: startWeekId,
		graceDays: graceDaysId
	};

	let startRule = $state<OnCallSettings['startRule']>('gestational_week');
	let startWeek = $state('37');
	let graceDays = $state('14');
	let isSaved = $state(false);
	const submission = new FormSubmission();

	const startRuleOptions: { value: OnCallSettings['startRule']; label: string; description: string }[] = [
		{
			value: 'gestational_week',
			label: 'At a week of pregnancy',
			description:
				'The usual answer. On call opens a fixed number of weeks into the pregnancy, counted back from the due date.'
		},
		{
			value: 'attachment_granted',
			label: 'From the day a doula takes the birth',
			description: 'For a practice that is on call from the moment it is hired.'
		}
	];

	function mapRefusal(refusal: unknown): FormError[] {
		return Array.isArray(refusal) ? refusal : errorsFromCause(refusal, fieldIds);
	}

	function apply(settings: OnCallSettings) {
		startRule = settings.startRule;
		startWeek = String(settings.startWeek);
		graceDays = String(settings.graceDays);
	}

	async function load() {
		try {
			apply(await loadOnCallSettings(apiFetchWithSession, page.params.practiceId!));
		} catch (error) {
			submission.errors = errorsFromCause(error);
		}
	}

	async function save() {
		isSaved = false;
		await submission.run(async () => {
			apply(
				await saveOnCallSettings(apiFetchWithSession, page.params.practiceId!, {
					startRule,
					// The BFF holds the bounds and says them in its own words, so
					// the screen sends what was typed rather than checking it
					// twice and risking two different sentences for one rule.
					startWeek: Number(startWeek),
					graceDays: Number(graceDays)
				})
			);
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
	<RadioGroup
		name={startRuleId}
		legend="When on call starts"
		options={startRuleOptions}
		value={startRule}
		onChange={(value) => (startRule = value)}
		error={submission.errorFor(startRuleId)}
	/>
	{#if startRule === 'gestational_week'}
		<LabeledField
			label="Week of pregnancy"
			hint="37 is the usual answer: on call opens three weeks before the due date."
			error={submission.errorFor(startWeekId)}
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="number"
					inputmode="numeric"
					value={startWeek}
					onInput={(value) => (startWeek = value)}
				/>
			{/snippet}
		</LabeledField>
	{/if}
	<LabeledField
		label="Days on call after the due date"
		hint="How long on call runs past the due date when no birth has been recorded yet. 14 covers a post-dates birth."
		error={submission.errorFor(graceDaysId)}
	>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				type="number"
				inputmode="numeric"
				value={graceDays}
				onInput={(value) => (graceDays = value)}
			/>
		{/snippet}
	</LabeledField>
	<!--
		Stated before the press, not reported after it (GOV.UK). Nothing
		stores an on-call window: it is worked out on every read from the
		due date and this rule, so saving this moves every window the
		Practice is carrying, including the one somebody is on call for
		tonight.
	-->
	<WarningText
		message="Changing this moves every on-call window this practice is carrying, including births already under way."
	/>
{/snippet}

{#snippet actions()}
	<Button label="Save" loading={submission.isSubmitting} onClick={save} />
{/snippet}

<FormPage
	title="On call"
	fieldsets={[{ content: fields }]}
	{actions}
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
/>

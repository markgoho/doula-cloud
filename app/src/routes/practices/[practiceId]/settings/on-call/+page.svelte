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
	 * birth without its own start at once, and the preview names which of
	 * today's births it would move.
	 */
	import { onMount, type Snippet } from 'svelte';
	import { afterNavigate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { formatActivityTimestamp } from '#lib/dates.js';
	import {
		describeOnCallEffect,
		describeSavedOnCall,
		isTypedRuleSavable,
		loadOnCallEffect,
		loadOnCallSettings,
		saveOnCallSettings,
		type OnCallSettings,
		type OnCallSettingsRead
	} from '#lib/onCallRule.js';
	import { practiceTimeZone } from '#lib/practiceZone.js';
	import { isOwnerOrAdmin } from '#lib/roles.js';
	import type { PracticeSession } from '../../+layout.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import { errorsFromCause, SERVICE_PROBLEM } from '#lib/formErrors.js';
	import { PageRead } from '#lib/pageRead.svelte.js';
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

	const practiceId = $derived(page.params.practiceId!);
	const session = $derived((page.data as { session: PracticeSession }).session);
	const canChangeRule = $derived(isOwnerOrAdmin(session));
	const rosterPath = $derived(resolve('/practices/[practiceId]/on-call', { practiceId }));
	const settingsHubPath = $derived(resolve('/practices/[practiceId]/settings', { practiceId }));

	let startRule = $state<OnCallSettings['startRule']>('gestational_week');
	let startWeek = $state('37');
	let graceDays = $state('14');
	// The sentence for the rule as the BFF saved it, not as the form reads
	// now: a change typed after the save has not been saved.
	let savedMessage = $state<string>();
	let effectLines = $state<string[]>([]);
	let rosterAddress = $state<string>();
	// Until the read answers the rule is unknown, and a read that failed is
	// not an answer: the page draws its failure with no Save, so the usual
	// answers can never be written over the Practice's rule.
	const ruleRead = new PageRead<OnCallSettingsRead>();
	const submission = new FormSubmission();

	const backHref = $derived(rosterAddress ?? settingsHubPath);
	const backLabel = $derived(rosterAddress ? 'On call' : 'Settings');

	afterNavigate(({ from }) => {
		rosterAddress =
			from?.url.pathname === rosterPath ? `${from.url.pathname}${from.url.search}` : undefined;
	});

	const startRuleOptions = $derived<
		{ value: OnCallSettings['startRule']; label: string; conditional?: Snippet }[]
	>([
		{ value: 'gestational_week', label: 'At a week of pregnancy', conditional: weekField },
		{ value: 'attachment_granted', label: 'From the day a Doula takes the birth' }
	]);

	function mapRefusal(refusal: unknown): FormError[] {
		return Array.isArray(refusal) ? refusal : errorsFromCause(refusal, fieldIds);
	}

	function apply(settings: OnCallSettings) {
		startRule = settings.startRule;
		startWeek = String(settings.startWeek);
		graceDays = String(settings.graceDays);
	}

	async function load() {
		await ruleRead.load(async () => {
			const settings = await loadOnCallSettings(apiFetchWithSession, practiceId);
			apply(settings);
			return settings;
		}, SERVICE_PROBLEM);
	}

	async function save() {
		savedMessage = undefined;
		await submission.run(async () => {
			const saved = await saveOnCallSettings(apiFetchWithSession, practiceId, {
				startRule,
				// The BFF holds the bounds and says them in its own words, so
				// the screen sends what was typed rather than checking it
				// twice and risking two different sentences for one rule.
				startWeek: Number(startWeek),
				graceDays: Number(graceDays)
			});
			apply(saved);
			ruleRead.set(saved);
			savedMessage = describeSavedOnCall(saved);
		}, mapRefusal);
	}

	// The preview asks only about a rule that could be saved and that is not
	// the one held, once typing has settled; a newer value drops the older
	// question and its answer.
	$effect(() => {
		const held = ruleRead.value;
		if (!held) return;
		const typed = { startRule, startWeek, graceDays };
		const isHeld =
			typed.startRule === held.startRule &&
			Number(typed.startWeek) === held.startWeek &&
			Number(typed.graceDays) === held.graceDays;
		if (isHeld || !isTypedRuleSavable(typed.startRule, typed.startWeek, typed.graceDays)) {
			effectLines = [];
			return;
		}
		const controller = new AbortController();
		const timer = setTimeout(async () => {
			try {
				const effect = await loadOnCallEffect(
					apiFetchWithSession,
					practiceId,
					{
						startRule: typed.startRule,
						startWeek: Number(typed.startWeek),
						graceDays: Number(typed.graceDays)
					},
					controller.signal
				);
				effectLines = describeOnCallEffect(effect);
			} catch {
				if (!controller.signal.aborted) effectLines = [];
			}
		}, 300);
		return () => {
			clearTimeout(timer);
			controller.abort();
		};
	});

	onMount(() => {
		if (canChangeRule) void load();
	});
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet weekField()}
	<LabeledField
		id={startWeekId}
		label="Week of pregnancy"
		hint="From 20 to 42. 37 is the usual answer: on call opens three weeks before the due date."
		error={submission.errorFor(startWeekId)}
		width={2}
	>
		<!-- A week of pregnancy is two digits (#1216); the label says "week" for a screen reader. -->
		{#snippet children({ id, describedBy, invalid, width })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				{width}
				suffix="weeks"
				type="text"
				inputmode="numeric"
				value={startWeek}
				onInput={(value) => (startWeek = value)}
			/>
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet readerFields()}
	<Notice variant="status" message="Only a Practice Owner or Admin can change the on-call rule." />
	<Link href={rosterPath} label="See who is on call" />
{/snippet}

{#snippet fields()}
	<RadioGroup
		name={startRuleId}
		legend="When on call starts"
		hint="One birth can start differently. Change it in the On call section of that birth's page."
		options={startRuleOptions}
		value={startRule}
		onChange={(value) => (startRule = value)}
		error={submission.errorFor(startRuleId)}
	/>
	<LabeledField
		id={graceDaysId}
		label="Days on call after the due date"
		hint="From 0 to 42. 14 covers a birth that comes after the due date."
		error={submission.errorFor(graceDaysId)}
		width={2}
	>
		<!-- At most 42 days (the API's maxGraceDays), so two digits (#1216). -->
		{#snippet children({ id, describedBy, invalid, width })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				{width}
				suffix="days"
				type="text"
				inputmode="numeric"
				value={graceDays}
				onInput={(value) => (graceDays = value)}
			/>
		{/snippet}
	</LabeledField>
	<div role="status" class="effect">
		{#each effectLines as line (line)}
			<Text text={line} />
		{/each}
	</div>
	<WarningText
		message="Saving this moves the on-call days of every birth this Practice is carrying, births on call today included. A birth with its own start keeps that start."
	/>
{/snippet}

{#snippet intro()}
	{#if ruleRead.value}
		{@const lastChanged = ruleRead.value.lastChanged}
		<p class="last-changed">
			{#if lastChanged}
				Last changed by {lastChanged.staffName},
				<time datetime={lastChanged.at}>{formatActivityTimestamp(lastChanged.at, practiceTimeZone())}</time>
			{:else}
				Not changed since this Practice was set up.
			{/if}
		</p>
	{/if}
{/snippet}

<!--
	#2291, after #2194: the save says the rule saved and which births follow
	it, under Save so the person who pressed it sees it without scrolling at
	320px. A polite status in place, never focused: Save is still there.
-->
{#snippet actions()}
	{#if canChangeRule}
		<stack-l space="var(--space-4)">
			<Button label="Save" loading={submission.isSubmitting} onClick={save} />
			{#if savedMessage}
				<Notice variant="status" message={savedMessage} />
			{/if}
		</stack-l>
	{/if}
{/snippet}

<FormPage
	title="On-call rule"
	{backHref}
	{backLabel}
	intro={canChangeRule ? intro : undefined}
	fieldsets={[{ content: canChangeRule ? fields : readerFields }]}
	{actions}
	read={canChangeRule ? ruleRead.frame('Loading the on-call rule') : undefined}
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
/>

<style>
	@layer components {
		.effect:empty {
			display: none;
		}

		.last-changed {
			margin: 0;
			font-family: var(--font-family-base);
			font-size: var(--text-meta-size);
			font-weight: var(--text-meta-weight);
			line-height: var(--text-meta-leading);
			letter-spacing: var(--text-meta-tracking);
			color: var(--color-on-surface-variant);
		}
	}
</style>

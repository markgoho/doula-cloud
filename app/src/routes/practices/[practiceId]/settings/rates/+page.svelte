<script lang="ts">
	/*
	 * A Practice's rate card (#966): the amount a new Contract is drafted
	 * at, per kind of Engagement.
	 *
	 * A form (#2030), so Enter in a box saves, and GOV.UK's Recover from
	 * validation errors pattern: both boxes are checked before anything is
	 * sent, and every refusal is said once in the `ErrorSummary` above the
	 * <h1> and again beside its box.
	 *
	 * A save says what was saved in a status `Notice`, read back from the
	 * server rather than from the boxes. A blank box is left alone -- #966's
	 * "no rate yet" is a valid state, and the BFF has no way to clear a
	 * rate once set -- so a box emptied over a stored rate keeps that rate,
	 * and the Notice and the refilled box say so rather than pretending it
	 * was removed. The Notice uses the Settings index's own words
	 * (`ratesValue`), so the two never describe one rate card two ways.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		loadRates,
		saveRate,
		dollarsToCents,
		centsToDollars,
		type EngagementKind,
		type Rate
	} from '#lib/rates.js';
	import { ratesValue } from '#lib/settingsSummary.js';
	import { FormSubmission, orThrownMessage, type FormError } from '#lib/formSubmission.svelte.js';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';

	const FIELD_IDS: Readonly<Record<EngagementKind, string>> = {
		birth: 'rate-birth',
		postpartum: 'rate-postpartum'
	};
	const KIND_WORDS: Readonly<Record<EngagementKind, string>> = { birth: 'birth', postpartum: 'postpartum' };

	let dollars = $state<Record<EngagementKind, string>>({ birth: '', postpartum: '' });
	let loadError = $state('');
	let isLoaded = $state(false);
	let savedMessage = $state('');
	const submission = new FormSubmission();

	function fill(rates: readonly Rate[]) {
		for (const rate of rates) {
			dollars[rate.kind] = rate.amountCents === null ? '' : centsToDollars(rate.amountCents);
		}
	}

	onMount(async () => {
		try {
			const { rates } = await loadRates(apiFetchWithSession, page.params.practiceId!);
			fill(rates);
			isLoaded = true;
		} catch (error) {
			loadError = error instanceof Error ? error.message : 'Failed to load rates';
		}
	});

	// Every box checked before anything is sent, so a refused postpartum
	// rate never leaves a birth rate half-saved behind it.
	function refusals(): FormError[] {
		return (Object.keys(FIELD_IDS) as EngagementKind[])
			.filter((kind) => dollars[kind].trim() !== '' && dollarsToCents(dollars[kind]) === undefined)
			.map((kind) => ({
				message: `Enter a ${KIND_WORDS[kind]} rate greater than $0, or leave it empty`,
				targetId: FIELD_IDS[kind]
			}));
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		savedMessage = '';
		await submission.run(async () => {
			const refused = refusals();
			if (refused.length > 0) return refused;
			for (const kind of Object.keys(FIELD_IDS) as EngagementKind[]) {
				const amountCents = dollarsToCents(dollars[kind]);
				if (amountCents !== undefined) {
					await saveRate(apiFetchWithSession, page.params.practiceId!, kind, amountCents);
				}
			}
			const { rates } = await loadRates(apiFetchWithSession, page.params.practiceId!);
			fill(rates);
			savedMessage = `Rates saved. ${ratesValue(rates)}.`;
		}, orThrownMessage);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet rateField(kind: EngagementKind, label: string, hint: string)}
	<!--
		A rate to the cent ("2500.00") is seven characters: GOV.UK's
		10-character step after a "$" prefix (#1216). The prefix is
		aria-hidden, as GOV.UK's is, so the hint says "in dollars" for a
		screen reader.
	-->
	<LabeledField id={FIELD_IDS[kind]} {label} {hint} error={submission.errorFor(FIELD_IDS[kind])} width={10}>
		{#snippet children({ id, describedBy, invalid, width })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				{width}
				prefix="$"
				type="number"
				step={0.01}
				min={0.01}
				inputmode="decimal"
				value={dollars[kind]}
				onInput={(value) => {
					dollars[kind] = value;
					savedMessage = '';
				}}
			/>
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet fields()}
	{#if savedMessage}
		<Notice variant="status" message={savedMessage} isFocusedOnAppear />
	{/if}
	<StackedForm onSubmit={save}>
		{@render rateField(
			'birth',
			'Birth rate',
			'In dollars. A new birth Contract is drafted at this amount. Leave it empty until you have a rate.'
		)}
		{@render rateField(
			'postpartum',
			'Postpartum rate',
			'In dollars. A new postpartum Contract is drafted at this amount. Leave it empty if you do not offer postpartum care.'
		)}
		<cluster-l space="var(--space-3)" align="center">
			<Button type="submit" label="Save" loading={submission.isSubmitting} />
		</cluster-l>
	</StackedForm>
{/snippet}

{#snippet actions()}{/snippet}

<FormPage
	title="Rates"
	fieldsets={[{ content: fields }]}
	{actions}
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	loading={isLoaded ? undefined : 'Loading your rates'}
	loadError={loadError || undefined}
/>

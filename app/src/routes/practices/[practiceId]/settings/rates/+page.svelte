<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadRates, saveRate, dollarsToCents, centsToDollars, type EngagementKind } from '#lib/rates.js';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';

	let birthDollars = $state('');
	let postpartumDollars = $state('');
	let birthError = $state('');
	let postpartumError = $state('');
	let loadError = $state('');
	let isSaved = $state(false);

	async function load() {
		loadError = '';
		isSaved = false;
		try {
			const { rates } = await loadRates(apiFetchWithSession, page.params.practiceId!);
			for (const rate of rates) {
				const dollars = rate.amountCents === null ? '' : centsToDollars(rate.amountCents);
				if (rate.kind === 'birth') {
					birthDollars = dollars;
				} else if (rate.kind === 'postpartum') {
					postpartumDollars = dollars;
				}
			}
		} catch (error) {
			loadError = error instanceof Error ? error.message : 'Failed to load rates';
		}
	}

	/**
	 * Saves one kind's rate. A blank field is left alone -- #966's AC that
	 * a Practice with no rate set is a valid state means this screen never
	 * forces a value into a kind nobody has priced yet. Returns the error
	 * message for that field, or '' on success -- caught here rather than
	 * left to propagate, so a failure on one kind never stops the other
	 * kind's own save.
	 */
	async function saveKind(kind: EngagementKind, dollars: string): Promise<string> {
		if (dollars.trim() === '') {
			return '';
		}
		const amountCents = dollarsToCents(dollars);
		if (amountCents === undefined) {
			return 'Enter an amount greater than zero';
		}
		try {
			await saveRate(apiFetchWithSession, page.params.practiceId!, kind, amountCents);
			return '';
		} catch (error) {
			return error instanceof Error ? error.message : 'Failed to save';
		}
	}

	async function save() {
		isSaved = false;
		birthError = await saveKind('birth', birthDollars);
		postpartumError = await saveKind('postpartum', postpartumDollars);
		isSaved = birthError === '' && postpartumError === '';
	}

	onMount(() => {
		void load();
	});
</script>

{#snippet fields()}
	{#if loadError}
		<Notice variant="error" message={loadError} />
	{/if}
	{#if isSaved}
		<Text text="Saved." />
	{/if}
	<LabeledField label="Birth rate (USD)" error={birthError}>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				type="number"
				step={0.01}
				min={0.01}
				value={birthDollars}
				onInput={(value) => (birthDollars = value)}
			/>
		{/snippet}
	</LabeledField>
	<LabeledField label="Postpartum rate (USD)" error={postpartumError}>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				type="number"
				step={0.01}
				min={0.01}
				value={postpartumDollars}
				onInput={(value) => (postpartumDollars = value)}
			/>
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet actions()}
	<Button label="Save" onClick={save} />
{/snippet}

<FormPage title="Rates" fieldsets={[{ content: fields }]} {actions} />

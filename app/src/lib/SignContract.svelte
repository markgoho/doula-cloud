<script lang="ts">
	import LabeledField from './components/molecules/LabeledField.svelte';
	import TextInput from './components/atoms/TextInput.svelte';
	import Checkbox from './components/atoms/Checkbox.svelte';

	let {
		onSign
	}: {
		onSign: (fullLegalName: string, isAttestation: boolean) => Promise<void>;
	} = $props();

	let isDisclosureAffirmed = $state(false);
	let fullLegalName = $state('');
	let isAttestation = $state(false);
	let error = $state('');
	let isSubmitting = $state(false);

	const canSubmit = $derived(fullLegalName.trim() !== '' && isAttestation && !isSubmitting);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		isSubmitting = true;
		try {
			await onSign(fullLegalName.trim(), isAttestation);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to sign';
		} finally {
			isSubmitting = false;
		}
	}
</script>

{#if !isDisclosureAffirmed}
	<section aria-labelledby="esign-disclosure-heading">
		<h2 id="esign-disclosure-heading">Electronic signature disclosure</h2>
		<p>By choosing to sign electronically, you consent to sign this Contract using an electronic signature.</p>
		<p>
			You have the right to receive a paper copy of this Contract instead. Contact your Practice if you'd
			like one.
		</p>
		<p>You may withdraw your consent to sign electronically at any time before you sign, with no penalty.</p>
		<button type="button" onclick={() => (isDisclosureAffirmed = true)}>
			I agree to sign electronically, continue
		</button>
	</section>
{:else}
	<form onsubmit={handleSubmit}>
		<LabeledField label="Full legal name">
			{#snippet children(control)}
				<TextInput
					value={fullLegalName}
					onInput={(value) => (fullLegalName = value)}
					required
					{...control}
				/>
			{/snippet}
		</LabeledField>
		<LabeledField label="I have read this Contract and I am signing it electronically" orientation="inline">
			{#snippet children(control)}
				<Checkbox checked={isAttestation} onChange={(checked) => (isAttestation = checked)} required {...control} />
			{/snippet}
		</LabeledField>
		{#if error}
			<p role="alert">{error}</p>
		{/if}
		<button type="submit" disabled={!canSubmit}>Sign</button>
	</form>
{/if}

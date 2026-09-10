<script lang="ts">
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Button from '#lib/components/atoms/Button.svelte';

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
	<section aria-labelledby="esign-disclosure-heading"> <!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
		<h2 id="esign-disclosure-heading">Electronic signature disclosure</h2>
		<p>By choosing to sign electronically, you consent to sign this Contract using an electronic signature.</p>
		<p>
			You have the right to receive a paper copy of this Contract instead. Contact your Practice if you'd
			like one.
		</p>
		<p>You may withdraw your consent to sign electronically at any time before you sign, with no penalty.</p>
		<Button
			label="I agree to sign electronically, continue"
			onClick={() => (isDisclosureAffirmed = true)}
		/>
	</section>
{:else}
	<!-- stacked-form:ignore: #1108 -- both controls are `required`, and that is the only thing standing between an unsigned name or an unticked attestation and the signing endpoint. `StackedForm` sets `novalidate` (ADR-0021), so adopting it here would take that refusal away and put nothing in its place; #1228 is where this form gets a refusal of its own and then adopts the molecule. The stack below is `StackedForm`'s own arrangement, written inline meanwhile. -->
	<form onsubmit={handleSubmit}>
		<stack-l space="var(--space-5)">
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
			<Button label="Sign" type="submit" disabled={!canSubmit} />
		</stack-l>
	</form>
{/if}

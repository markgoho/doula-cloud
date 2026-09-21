<script lang="ts">
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import { FormSubmission, orThrownMessage } from '#lib/formSubmission.svelte.js';
	import type { FormError } from '#lib/formErrors.js';

	let {
		onSign
	}: {
		onSign: (fullLegalName: string, isAttestation: boolean) => Promise<void>;
	} = $props();

	let isDisclosureAffirmed = $state(false);
	let fullLegalName = $state('');
	let isAttestation = $state(false);

	// #1228: both controls carried `required`, the only refusal this form
	// had -- StackedForm's `novalidate` (ADR-0021) takes that away, so the
	// same two checks that used to gate the Sign button (`canSubmit`) move
	// here instead. Refusing on submit rather than disabling the button
	// keeps the same pattern every other form in the app uses: a control
	// that is always reachable, and a summary that says why it refused.
	const fullLegalNameFieldId = 'sign-contract-full-legal-name';
	const attestationFieldId = 'sign-contract-attestation';

	const signSubmission = new FormSubmission();

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		await signSubmission.run(async () => {
			const errors: FormError[] = [];
			if (fullLegalName.trim() === '') {
				errors.push({ message: 'Enter your full legal name', targetId: fullLegalNameFieldId });
			}
			if (!isAttestation) {
				errors.push({
					message: 'Confirm that you have read the Contract and are signing it electronically',
					targetId: attestationFieldId
				});
			}
			if (errors.length > 0) return errors;

			await onSign(fullLegalName.trim(), isAttestation);
		}, orThrownMessage);
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
	<StackedForm onSubmit={handleSubmit}>
		{#if signSubmission.errors.length > 0}
			<ErrorSummary errors={signSubmission.errors} />
		{/if}
		<LabeledField
			id={fullLegalNameFieldId}
			label="Full legal name"
			error={signSubmission.errorFor(fullLegalNameFieldId)}
		>
			{#snippet children(control)}
				<TextInput
					value={fullLegalName}
					onInput={(value) => (fullLegalName = value)}
					required
					{...control}
				/>
			{/snippet}
		</LabeledField>
		<LabeledField
			id={attestationFieldId}
			label="I have read this Contract and I am signing it electronically"
			orientation="inline"
			error={signSubmission.errorFor(attestationFieldId)}
		>
			{#snippet children(control)}
				<Checkbox checked={isAttestation} onChange={(checked) => (isAttestation = checked)} required {...control} />
			{/snippet}
		</LabeledField>
		<Button label="Sign" type="submit" loading={signSubmission.isSubmitting} />
	</StackedForm>
{/if}

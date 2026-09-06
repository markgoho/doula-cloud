<script lang="ts">
	/*
	 * #619, ADR-0026: she changes her own sign-in address, and the change
	 * is proved by a link sent to the new address.
	 *
	 * Two things about the shape are decisions, not accidents.
	 *
	 * The page never reports whether the address she typed is already in
	 * use. The BFF answers the same way either way (#168's
	 * account-enumeration class), so a screen that said "that one is
	 * taken" would be inventing a fact it was deliberately not told. The
	 * collision surfaces on the confirmation screen instead, where she has
	 * proved the mailbox and so learns nothing new.
	 *
	 * The outcome is announced in place with a Notice and the form stays
	 * where it is -- docs/design/govuk-alignment.md's second recorded
	 * departure, "No confirmation pages". A second submit after that is
	 * not a hazard to design around: asking again supersedes the first
	 * link outright (authtoken.Mint deletes the prior token and the
	 * companion row cascades with it), which is what she wants if she
	 * mistyped the address.
	 */
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import { refusalErrors, SERVICE_PROBLEM, type FormError } from '#lib/formErrors.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';

	const fieldId = 'new-sign-in-address';

	let currentAddress = $state('');
	let isLoaded = $state(false);
	let loadError = $state('');
	let newAddress = $state('');
	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);
	let sentTo = $state('');

	const fieldError = $derived(errors.find((error) => error.targetId === fieldId)?.message);

	// onMount, not $effect: this reads once, and an effect would re-run
	// every time it set `isLoaded` -- the sibling Notifications screen
	// loads its own state the same way.
	onMount(loadCurrentAddress);

	async function loadCurrentAddress() {
		try {
			const response = await apiFetchWithSession('/api/portal/session');
			if (!response.ok) {
				loadError = SERVICE_PROBLEM;
				return;
			}
			const session: { signInAddress?: string } = await response.json();
			currentAddress = session.signInAddress ?? '';
			isLoaded = true;
		} catch {
			loadError = SERVICE_PROBLEM;
		}
	}

	// The same two checks the BFF makes, made here first so the common
	// mistakes never cost a round trip. The BFF still makes them: this is
	// a convenience, not the enforcement (ADR-0024's client/server split
	// applied to validation -- prevent on the client, refuse on the
	// server).
	function localError(address: string): string {
		if (address.trim() === '') return 'Enter your new sign-in address';
		const at = address.trim().indexOf('@');
		if (at <= 0 || at >= address.trim().length - 1) {
			return 'Enter an email address in the correct format, like name@example.com';
		}
		return '';
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errors = [];

		const problem = localError(newAddress);
		if (problem) {
			errors = [{ message: problem, targetId: fieldId }];
			return;
		}

		isSubmitting = true;
		try {
			const response = await apiFetchWithSession('/api/portal/sign-in-address/request', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email: newAddress.trim() })
			});
			if (!response.ok) {
				errors = await refusalErrors(response, { email: fieldId });
				return;
			}
			sentTo = newAddress.trim();
		} catch {
			errors = [{ message: SERVICE_PROBLEM }];
		} finally {
			isSubmitting = false;
		}
	}
</script>

{#snippet intro()}
	<Text
		text={`You sign in with ${currentAddress}. This is your own address, and it is separate from the contact details your Practice holds for you.`}
	/>
	<Text
		text="We will send a link to the new address. Your current address keeps signing you in until you open it."
		tone="variant"
	/>
{/snippet}

{#snippet field()}
	<LabeledField
		id={fieldId}
		label="New sign-in address"
		hint="You will need to open an email at this address to finish the change."
		error={fieldError}
	>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				type="email"
				name="email"
				autocomplete="email"
				value={newAddress}
				onInput={(value) => (newAddress = value)}
			/>
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

{#snippet actions()}
	<Button type="submit" label="Send the link" loading={isSubmitting} />
	{#if sentTo}
		<Notice
			variant="status"
			message={`Check your email. Open the link we sent to ${sentTo} to finish the change. Until you do, ${currentAddress} still signs you in.`}
		/>
	{/if}
{/snippet}

<!--
	novalidate is GOV.UK's own instruction, not a shortcut past HTML
	validation: the browser's built-in bubble is announced inconsistently,
	disappears on the next keystroke, and cannot be summarized at the top
	of the page. type="email" stays, because it still gets her the right
	keyboard on a phone; what it no longer does is speak for the service.
-->
<form onsubmit={handleSubmit} novalidate>
	<FormPage
		title="Sign-in address"
		serviceName={page.data.practiceName}
		{intro}
		fieldsets={[{ content: field }]}
		errorSummary={errors.length > 0 ? errorSummary : undefined}
		{actions}
		loading={isLoaded || loadError ? undefined : 'Loading your sign-in address'}
		{loadError}
	/>
</form>

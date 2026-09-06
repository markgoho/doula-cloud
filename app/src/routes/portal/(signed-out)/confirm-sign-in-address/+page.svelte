<script lang="ts">
	/*
	 * #619, ADR-0026: the second half of a sign-in-address change --
	 * proving the new mailbox.
	 *
	 * In the signed-out group, not the authenticated one, because the link
	 * arrives in the new mailbox and may be opened on a device that has
	 * never signed in. The token is the whole credential, so the screen
	 * needs no session and must not demand one.
	 *
	 * The token is spent on this POST and never on the GET that rendered
	 * the page -- the same scanner rule the sign-in screen keeps: a mail
	 * client following the link to inspect it must not burn it before she
	 * reads the mail.
	 */
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiBaseURL } from '#lib/api.js';
	import { refusalErrors, SERVICE_PROBLEM, type FormError } from '#lib/formErrors.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	const token = page.url.searchParams.get('token') ?? '';

	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);
	let confirmedAddress = $state('');

	async function handleContinue() {
		errors = [];
		isSubmitting = true;
		try {
			const response = await fetch(`${apiBaseURL()}/api/portal/sign-in-address`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token })
			});
			if (!response.ok) {
				errors = await refusalErrors(response);
				return;
			}
			const body: { signInAddress: string } = await response.json();
			confirmedAddress = body.signInAddress;
		} catch {
			errors = [{ message: SERVICE_PROBLEM }];
		} finally {
			isSubmitting = false;
		}
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

{#snippet content()}
	{#if !token}
		<Notice variant="error" message="This link is missing its confirmation code." />
	{:else if confirmedAddress}
		<Notice variant="status" message="Your sign-in address has changed" />
		<Text text={`From now on you sign in with ${confirmedAddress}. Your old address no longer works.`} />
		<Link href={resolve('/portal/(signed-out)/login')} label="Sign in" />
	{:else}
		<Text text="Confirm that you want to use this address to sign in to Doula Cloud." />
		<Button type="button" label="Continue" loading={isSubmitting} onClick={handleContinue} />
	{/if}
{/snippet}

<EntryPage
	title="Confirm your sign-in address"
	errorSummary={errors.length > 0 ? errorSummary : undefined}
	{content}
/>

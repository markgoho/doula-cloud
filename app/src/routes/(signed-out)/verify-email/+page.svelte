<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiBaseURL } from '#lib/api.js';
	import { refusalMessage, SERVICE_PROBLEM } from '#lib/formErrors.js';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	const token = page.url.searchParams.get('token') ?? '';

	let status = $state<'checking' | 'verified' | 'failed'>('checking');
	let errorMessage = $state('');

	onMount(async () => {
		if (!token) {
			status = 'failed';
			errorMessage = 'This link is missing its verification code.';
			return;
		}
		try {
			const response = await fetch(`${apiBaseURL()}/api/staff/verify-email`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token })
			});
			if (!response.ok) {
				errorMessage = await refusalMessage(response);
				status = 'failed';
				return;
			}
			// #613: the cached ID token still says email_verified: false
			// until it's force-refreshed. Only matters if a live Firebase
			// user exists here, which is the exception, not the rule -- every
			// bootstrap flow (login, signup, accept-invite) signs out of the
			// Firebase SDK right after exchanging for the __session cookie,
			// so #606's own enrollment flow (not built yet) will need to
			// re-establish a live Firebase user and refresh again itself
			// before reading email_verified, rather than relying on this.
			const user = getFirebaseAuth().currentUser;
			if (user) {
				await user.reload();
				await user.getIdToken(true);
			}
			status = 'verified';
		} catch {
			errorMessage = SERVICE_PROBLEM;
			status = 'failed';
		}
	});
</script>

<!--
	No `errorSummary`, so no "Error: " title prefix when the link fails
	(#1576): a failed link is not a refused form, and govuk-alignment.md's
	title-format rule keeps that prefix for refused forms only.
-->
<EntryPage title="Verify your email">
	{#snippet content()}
		{#if status === 'checking'}
			<Notice variant="info" message="Checking your link…" />
		{:else if status === 'verified'}
			<Notice variant="status" message="Your email address is verified." />
			<Link href={resolve('/(signed-out)/login')} label="Continue to log in" />
		{:else}
			<Notice variant="error" message={errorMessage} />
		{/if}
	{/snippet}
</EntryPage>

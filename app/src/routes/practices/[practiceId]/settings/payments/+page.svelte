<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import { loadConnectStatus, connect, type ConnectStatus, type ConnectStatusResult } from '#lib/payments.js';

	let status = $state<ConnectStatusResult | undefined>();
	let error = $state('');
	let roles = $state<string[]>([]);
	let isOwner = $derived(roles.includes('owner'));
	let connectParameter = $derived(page.url.searchParams.get('connect'));

	let connectError = $state('');
	let isConnecting = $state(false);

	onMount(async () => {
		const user = getFirebaseAuth().currentUser;
		if (!user) {
			error = 'You must be logged in to view payments settings';
			return;
		}
		const idToken = await user.getIdToken();

		try {
			const fetch = (path: string, init?: RequestInit) => apiFetch(path, idToken, init);
			status = await loadConnectStatus(fetch, page.params.practiceId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Stripe Connect status';
			return;
		}

		// The connect button's enabled state mirrors the "owner"-role gating
		// the billing page already uses -- server-side enforcement
		// (RequireOwner) is what actually matters, this is UX only.
		const sessionResponse = await apiFetch(`/api/practices/${page.params.practiceId}/session`, idToken);
		if (sessionResponse.ok) {
			const body: { roles: string[] } = await sessionResponse.json();
			roles = body.roles;
		}
	});

	async function handleConnect() {
		connectError = '';
		isConnecting = true;
		try {
			const user = getFirebaseAuth().currentUser;
			if (!user) {
				connectError = 'You must be logged in to connect Stripe';
				return;
			}
			const idToken = await user.getIdToken();
			const fetch = (path: string, init?: RequestInit) => apiFetch(path, idToken, init);
			const onboardingUrl = await connect(fetch, page.params.practiceId!);
			location.assign(onboardingUrl);
		} catch (error_) {
			connectError = error_ instanceof Error ? error_.message : 'Failed to start Stripe Connect onboarding';
		} finally {
			isConnecting = false;
		}
	}

	const statusLabels: Record<ConnectStatus, string> = {
		not_connected: 'Not connected',
		onboarding_incomplete: 'Onboarding incomplete',
		active: 'Active'
	};
</script>

<h1>Payments</h1>

{#if error}
	<p role="alert">{error}</p>
{:else if status}
	<p>Stripe Connect status: {statusLabels[status.status]}</p>

	{#if connectParameter === 'return'}
		<p role="status">Stripe onboarding finished. Status updates once Stripe confirms your account is active.</p>
	{:else if connectParameter === 'refresh'}
		<p role="status">Your Stripe onboarding link expired. Start again below.</p>
	{/if}

	{#if isOwner && status.status !== 'active'}
		<button type="button" onclick={handleConnect} disabled={isConnecting}>
			{status.status === 'not_connected' ? 'Connect Stripe' : 'Continue Stripe onboarding'}
		</button>
		{#if connectError}
			<p role="alert">{connectError}</p>
		{/if}
	{:else if !isOwner && status.status !== 'active'}
		<p>Ask a Practice Owner to connect Stripe.</p>
	{/if}
{/if}

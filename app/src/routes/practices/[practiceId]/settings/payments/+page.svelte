<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadConnectStatus, connect, type ConnectStatus, type ConnectStatusResult } from '#lib/payments.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';

	let status = $state<ConnectStatusResult | undefined>();
	let error = $state('');
	let roles = $state<string[]>([]);
	let isOwner = $derived(roles.includes('owner'));
	let connectParameter = $derived(page.url.searchParams.get('connect'));

	let connectError = $state('');
	let isConnecting = $state(false);

	onMount(async () => {
		try {
			status = await loadConnectStatus(apiFetchWithSession, page.params.practiceId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Stripe Connect status';
			return;
		}

		// The connect button's enabled state mirrors the "owner"-role gating
		// the billing page already uses -- server-side enforcement
		// (RequireOwner) is what actually matters, this is UX only.
		const sessionResponse = await apiFetchWithSession(`/api/practices/${page.params.practiceId}/session`);
		if (sessionResponse.ok) {
			const body: { roles: string[] } = await sessionResponse.json();
			roles = body.roles;
		}
	});

	async function handleConnect() {
		connectError = '';
		isConnecting = true;
		try {
			const onboardingUrl = await connect(apiFetchWithSession, page.params.practiceId!);
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

	const statusBadgeVariants: Record<ConnectStatus, 'neutral' | 'warning' | 'success'> = {
		not_connected: 'neutral',
		onboarding_incomplete: 'warning',
		active: 'success'
	};
</script>

<Heading level={1} text="Payments" />

{#if error}
	<Notice variant="error" message={error} />
{:else if status}
	<cluster-l>
		<Text text="Stripe Connect status:" />
		<Badge label={statusLabels[status.status]} variant={statusBadgeVariants[status.status]} />
	</cluster-l>

	{#if connectParameter === 'return'}
		<Notice
			variant="status"
			message="Stripe onboarding finished. Status updates once Stripe confirms your account is active."
		/>
	{:else if connectParameter === 'refresh'}
		<Notice variant="status" message="Your Stripe onboarding link expired. Start again below." />
	{/if}

	{#if isOwner && status.status !== 'active'}
		<Button
			label={status.status === 'not_connected' ? 'Connect Stripe' : 'Continue Stripe onboarding'}
			onClick={handleConnect}
			loading={isConnecting}
		/>
		{#if connectError}
			<Notice variant="error" message={connectError} />
		{/if}
	{:else if !isOwner && status.status !== 'active'}
		<Text text="Ask a Practice Owner to connect Stripe." />
	{/if}
{/if}

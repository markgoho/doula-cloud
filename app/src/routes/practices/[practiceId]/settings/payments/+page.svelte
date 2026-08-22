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
		pending: 'Awaiting Stripe review',
		payouts_restricted: 'Taking payments, payouts on hold',
		active: 'Active'
	};

	const statusBadgeVariants: Record<ConnectStatus, 'neutral' | 'warning' | 'success'> = {
		not_connected: 'neutral',
		onboarding_incomplete: 'warning',
		pending: 'warning',
		payouts_restricted: 'warning',
		active: 'success'
	};

	// What each status means for the Practice, in the Owner's terms rather
	// than Stripe's. `pending` is the one v1 could not report at all: the
	// Owner has finished, so offering them the onboarding button again
	// would be misleading.
	const statusExplanations: Record<ConnectStatus, string> = {
		not_connected: 'Connect Stripe so Clients can pay their invoices.',
		onboarding_incomplete: 'Stripe still needs some details before Clients can pay you.',
		pending: 'Stripe is reviewing the details you submitted. Nothing is needed from you.',
		payouts_restricted:
			'Clients can pay their invoices, but Stripe cannot send the money to your bank yet.',
		active: 'Clients can pay their invoices and payouts reach your bank.'
	};

	// The onboarding button only helps when there is something the Owner
	// can actually supply. While Stripe is reviewing, there is not.
	let canStartOnboarding = $derived(
		status !== undefined && status.status !== 'active' && status.status !== 'pending'
	);
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

	<Text text={statusExplanations[status.status]} />

	{#if status.requirementsDue.length > 0}
		<Text text="Stripe is still waiting on:" />
		<ul>
			{#each status.requirementsDue as requirement (requirement)}
				<li>{requirement}</li>
			{/each}
		</ul>
	{/if}

	{#if isOwner && canStartOnboarding}
		<Button
			label={status.status === 'not_connected' ? 'Connect Stripe' : 'Continue Stripe onboarding'}
			onClick={handleConnect}
			loading={isConnecting}
		/>
		{#if connectError}
			<Notice variant="error" message={connectError} />
		{/if}
	{:else if !isOwner && canStartOnboarding}
		<Text text="Ask a Practice Owner to connect Stripe." />
	{/if}
{/if}

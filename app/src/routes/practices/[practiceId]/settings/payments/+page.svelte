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

	// One row per status rather than four parallel maps: the label, the
	// badge, what it means for the Practice in the Owner's words, and
	// whether reopening Stripe's hosted form could help.
	//
	// `onboarding` is not derivable from the status alone. `pending` means
	// Stripe is reviewing and there is nothing to supply, so offering the
	// button would be a dead end -- but `payouts_restricted` can mean
	// either (Stripe reviewing the bank details, or the Owner never
	// entered any), so that one asks whether anything is outstanding.
	const statusCopy: Record<
		ConnectStatus,
		{
			label: string;
			variant: 'neutral' | 'warning' | 'success';
			explanation: string;
			onboarding: 'always' | 'never' | 'if-outstanding';
		}
	> = {
		not_connected: {
			label: 'Not connected',
			variant: 'neutral',
			explanation: 'Connect Stripe so Clients can pay their invoices.',
			onboarding: 'always'
		},
		onboarding_incomplete: {
			label: 'Onboarding incomplete',
			variant: 'warning',
			explanation: 'Stripe still needs some details before Clients can pay you.',
			onboarding: 'always'
		},
		pending: {
			label: 'Awaiting Stripe review',
			variant: 'warning',
			explanation: 'Stripe is reviewing the details you submitted. Nothing is needed from you.',
			onboarding: 'never'
		},
		payouts_restricted: {
			label: 'Taking payments, payouts on hold',
			variant: 'warning',
			explanation:
				'Clients can pay their invoices, but Stripe cannot send the money to your bank yet.',
			onboarding: 'if-outstanding'
		},
		active: {
			label: 'Active',
			variant: 'success',
			explanation: 'Clients can pay their invoices and payouts reach your bank.',
			onboarding: 'never'
		}
	};

	let copy = $derived(status === undefined ? undefined : statusCopy[status.status]);

	let canStartOnboarding = $derived(
		copy?.onboarding === 'always' ||
			(copy?.onboarding === 'if-outstanding' && (status?.requirementsDue.length ?? 0) > 0)
	);

</script>

<Heading level={1} text="Payments" />

{#if error}
	<Notice variant="error" message={error} />
{:else if status}
	<cluster-l>
		<Text text="Stripe Connect status:" />
		<Badge label={statusCopy[status.status].label} variant={statusCopy[status.status].variant} />
	</cluster-l>

	{#if connectParameter === 'return'}
		<Notice
			variant="status"
			message="Stripe onboarding finished. Status updates once Stripe confirms your account is active."
		/>
	{:else if connectParameter === 'refresh'}
		<Notice variant="status" message="Your Stripe onboarding link expired. Start again below." />
	{/if}

	<Text text={statusCopy[status.status].explanation} />

	<!-- The count, not the list. requirementsDue holds Stripe's own
	machine-readable field paths ("configuration.merchant.mcc"), which
	name nothing an Owner recognizes. The place those get asked in words
	is Stripe's hosted form, which the button below opens; the paths stay
	in the database for the audit trail. -->
	{#if status.requirementsDue.length > 0}
		<Text
			text={status.requirementsDue.length === 1
				? 'Stripe needs 1 more detail from you.'
				: `Stripe needs ${status.requirementsDue.length} more details from you.`}
		/>
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

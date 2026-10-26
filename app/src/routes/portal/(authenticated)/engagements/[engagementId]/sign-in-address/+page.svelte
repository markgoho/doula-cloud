<script lang="ts">
	/*
	 * #619, ADR-0026: the address she signs in with, and the way to change
	 * it, which is proved by a link sent to the new address.
	 *
	 * A summary, not a form (#2045): the address is one SummaryList row
	 * whose Change opens a question page of its own (./change), which
	 * sends the link and comes back here with a status Notice that says
	 * so. "Sign out of every device" is its own last section, apart from
	 * the row, because it is a separate act with no relation to the
	 * address. It was one FormPage whose second fieldset was the sign-out,
	 * so the "Send the link" button landed under the red one.
	 */
	import { onMount } from 'svelte';
	import { goto, refreshAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetch, apiFetchWithSession } from '#lib/api.js';
	import { apiErrorMessage } from '#lib/apiErrorMessage.js';
	import { SERVICE_PROBLEM } from '#lib/formErrors.js';
	import { takeOutcome } from '#lib/outcome.js';
	import { portalPushSubscriptionsPath, unregisterPushSubscription } from '#lib/pushRegistration.js';
	import { bestEffort, UNREGISTER_TIMEOUT_MS } from '#lib/signOut.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';

	// Read while the page renders, before `goto` settles (#1710): what the
	// change question sent, said once.
	const outcome = takeOutcome(page.url);

	let currentAddress = $state('');
	let isLoaded = $state(false);
	let loadError = $state('');

	// #618, ADR-0026: her own "sign out everywhere", a separate act from
	// the address change -- its own section, its own confirmation.
	let isSignOutEverywhereDialogOpen = $state(false);
	let isSigningOutEverywhere = $state(false);
	let signOutEverywhereError = $state('');

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

	// Ends every session on every device, this one included (#618) -- so
	// a successful call lands her back on the login screen rather than
	// leaving this page rendered behind a cookie that no longer verifies.
	//
	// Takes this device off push first, the same order and the same
	// bounded best-effort signOut.ts's own signOutOfSession uses for
	// ordinary sign-out, and for the same reason: the unregister endpoint
	// authenticates by the session this call is about to end, so run
	// second and it 401s, leaving whoever picks this device up later
	// reading her pushes on the lock screen.
	async function handleSignOutEverywhere() {
		signOutEverywhereError = '';
		isSigningOutEverywhere = true;
		try {
			await bestEffort(
				() => unregisterPushSubscription(portalPushSubscriptionsPath(page.params.engagementId!), apiFetch),
				UNREGISTER_TIMEOUT_MS
			);
			const response = await apiFetchWithSession('/api/portal/sessions', { method: 'DELETE' });
			if (!response.ok) {
				throw new Error(await apiErrorMessage(response));
			}
			// engagements/[engagementId]/+layout.ts's load result is keyed
			// on params alone, so a Back press to this exact URL would
			// otherwise reuse the still-signed-in load instead of
			// re-checking the session (#487) -- the same reason the portal
			// layout's own sign-out invalidates before it navigates.
			await refreshAll();
			await goto(resolve('/portal/(signed-out)/login'));
		} catch (error_) {
			// Rethrown so ConfirmDialog stays open and renders this inside
			// itself (#804), rather than closing over a failure she has had
			// no chance to read.
			signOutEverywhereError =
				error_ instanceof Error ? error_.message : 'Failed to sign out of every device';
			throw error_;
		} finally {
			isSigningOutEverywhere = false;
		}
	}
</script>

{#snippet summary()}
	<stack-l space="var(--space-3)">
		<SummaryList
			isBordered
			rows={[
				{
					label: 'Sign-in address',
					value: currentAddress,
					changes: 'sign-in address',
					href: resolve('/portal/(authenticated)/engagements/[engagementId]/sign-in-address/change', {
						engagementId: page.params.engagementId!
					})
				}
			]}
		>
			{#snippet action(row, describedBy)}
				<Link href={row.href} label="Change" {describedBy} />
			{/snippet}
		</SummaryList>
		<Text
			text="This is your own address. It is separate from the contact details your Practice holds for you."
			step="body-sm"
			tone="muted"
		/>
	</stack-l>
{/snippet}

{#snippet signOutEverywhere()}
	<WarningText
		message="This ends every session on every device, including this one. You will need to sign in again afterwards."
	/>
	<Button
		type="button"
		label="Sign out of every device"
		variant="destructive"
		loading={isSigningOutEverywhere}
		onClick={() => (isSignOutEverywhereDialogOpen = true)}
	/>
	<ConfirmDialog
		bind:open={isSignOutEverywhereDialogOpen}
		title="Sign out of every device"
		consequence="You are signed out on every device immediately, including this one."
		confirmLabel="Sign out of every device"
		error={signOutEverywhereError}
		onConfirm={handleSignOutEverywhere}
	/>
{/snippet}

<RecordDetail
	title="Sign-in address"
	serviceName={page.data.practiceName}
	{outcome}
	{summary}
	sections={[{ heading: 'Sign out of every device', content: signOutEverywhere }]}
	loading={isLoaded || loadError ? undefined : 'Loading your sign-in address'}
	loadError={loadError || undefined}
/>

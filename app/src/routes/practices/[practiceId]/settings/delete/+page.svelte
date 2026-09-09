<script lang="ts">
	/*
	 * #871: a Practice deleting itself. ADR-0031 is the rule this screen
	 * follows -- redact in place, a 30-day restore window, cascade every
	 * Client's own erasure at day 30, forfeit any unspent Credit balance.
	 *
	 * Shaped like settings/mfa/+page.svelte: an Owner-only FormPage that
	 * loads its own status and gates the one consequential action behind
	 * ConfirmDialog. Unlike that screen, the two states (nothing pending,
	 * a window open) are different enough to read as two bodies rather
	 * than one toggle -- restoring is deliberately not behind a
	 * ConfirmDialog of its own, the same reasoning "stop requiring MFA"
	 * sends nothing to confirm: it undoes the destructive act rather than
	 * committing to it.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { isOwner as checkIsOwner } from '#lib/roles.js';
	import {
		loadDeletionStatus,
		initiateDeletion,
		restorePractice,
		type DeletionStatus
	} from '#lib/practiceDeletion.js';
	import { formatInstant } from '#lib/dates.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import type { PracticeSession } from '../../+layout.js';

	const session = $derived((page.data as { session: PracticeSession }).session);
	let isOwner = $derived(checkIsOwner(session));

	let status = $state<DeletionStatus | undefined>();
	let loadError = $state('');
	let actionError = $state('');
	let successNotice = $state('');
	let isConfirmOpen = $state(false);
	let isSubmitting = $state(false);

	onMount(async () => {
		if (isOwner) {
			await loadStatus();
		}
	});

	async function loadStatus() {
		loadError = '';
		try {
			status = await loadDeletionStatus(apiFetchWithSession, page.params.practiceId!);
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load this setting';
		}
	}

	async function handleConfirmDelete() {
		actionError = '';
		try {
			await initiateDeletion(apiFetchWithSession, page.params.practiceId!);
			successNotice = '';
			await loadStatus();
		} catch (error_) {
			actionError = error_ instanceof Error ? error_.message : 'Failed to start deletion';
			throw error_;
		}
	}

	async function handleRestore() {
		actionError = '';
		isSubmitting = true;
		try {
			await restorePractice(apiFetchWithSession, page.params.practiceId!);
			successNotice = 'This Practice has been restored.';
			await loadStatus();
		} catch (error_) {
			actionError = error_ instanceof Error ? error_.message : 'Failed to restore this Practice';
		} finally {
			isSubmitting = false;
		}
	}

	// ADR-0031's own wording: what is destroyed, what survives, and the
	// window before either happens. Names two of Doula Cloud's own
	// business records -- the Credit ledger and the Stripe Connect
	// account -- as kept, the plain-language stand-ins for the full
	// AC #4 list (which also includes Stripe webhook events, too far
	// into implementation detail for an Owner-facing confirmation to
	// name by that word). They are Doula Cloud's own obligation to
	// retain, not the Practice's to delete out from under.
	let deleteConsequence = $derived(
		`This starts a 30-day countdown to delete ${session.practiceName}. During it, no Staff member or Client can sign in to this Practice, except that any Owner can return here to restore it. If the countdown finishes, every Client on file has their personal data erased the same way a single Client's own erasure works, any unspent Credit balance is forfeited, and this cannot be undone. Doula Cloud's own billing records -- the Credit ledger, the Stripe Connect account and its payouts -- are kept regardless.`
	);

	let loading = $derived(isOwner && status === undefined ? 'Loading this setting' : undefined);
</script>

{#snippet intro()}
	{#if !isOwner}
		<!-- Notice renders in body below; intro stays generic here. -->
	{:else if status?.pending}
		<Badge label="Deletion pending" variant="warning" />
	{/if}
{/snippet}

{#snippet body()}
	{#if !isOwner}
		{#if session.pendingDeletion}
			<Notice
				variant="status"
				message="This Practice is scheduled for deletion. Only an Owner can restore it."
			/>
		{:else}
			<Notice variant="status" message="Only a Practice Owner can view or change this setting." />
		{/if}
	{:else if status?.pending}
		<Notice
			variant="status"
			message={`This Practice is scheduled to be deleted on ${formatInstant(status.finalizeAt!)}. Restore it before then to keep it.`}
		/>
	{:else if status?.hasUnsettledInvoices}
		<Notice
			variant="info"
			message="This Practice can't be deleted yet -- it has an unsettled Invoice. Settle or void it first."
		/>
	{/if}
	{#if successNotice}
		<Notice variant="status" message={successNotice} />
	{/if}
	{#if actionError && !isConfirmOpen}
		<!-- Delete's own failure renders inside ConfirmDialog while it is
		     open (#804); this is Restore's, which has no dialog to gate it. -->
		<Notice variant="error" message={actionError} />
	{/if}
{/snippet}

{#snippet actions()}
	{#if isOwner && status}
		{#if status.pending}
			<Button label="Restore this Practice" loading={isSubmitting} onClick={handleRestore} />
		{:else if !status.hasUnsettledInvoices}
			<Button
				label="Delete this Practice"
				variant="destructive"
				onClick={() => (isConfirmOpen = true)}
			/>
		{/if}
	{/if}
{/snippet}

<FormPage title="Delete this Practice" {intro} fieldsets={[{ content: body }]} {actions} {loading} {loadError} />

<ConfirmDialog
	bind:open={isConfirmOpen}
	title={`Delete ${session.practiceName}`}
	consequence={deleteConsequence}
	confirmLabel="Delete this Practice"
	error={actionError}
	onConfirm={handleConfirmDelete}
/>

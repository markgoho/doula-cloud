<script lang="ts">
	/*
	 * Where an Owner raises MFA from "optional for every Staff member" to
	 * "mandatory for every Staff member" (#606). Until #1531 an Owner was
	 * always held to it; since then she is held to it only by this switch
	 * and by the five acts that need it (#1532).
	 *
	 * The PUT below refuses without an X-Confirmed header (400) on the
	 * theory that the client has already shown what it is about to do --
	 * so turning the switch on follows GOV.UK's confirmation-page pattern
	 * (docs/design/govuk-alignment.md, ADR-0021) and names how many Staff
	 * have no second factor yet, because those are the accounts that stop
	 * being able to sign in the moment this saves. Turning it off bars
	 * nobody, so it saves straight away -- the same shape payments'
	 * "Connect Stripe" button already uses for a consequential action with
	 * nothing to confirm.
	 *
	 * The GET this reads is Owner-only server-side, one notch narrower
	 * than payments' status endpoint (Owner or Admin, #267) and narrower
	 * still than website's, which any Staff member may read. So a
	 * non-Owner is never asked for it: she sees a notice and nothing about
	 * the setting itself, rather than a status everyone else's settings
	 * screens show with only the button removed.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { hasSecondFactor as checkHasSecondFactor, isOwner as checkIsOwner } from '#lib/roles.js';
	import {
		loadMfaRequirementImpact,
		setMfaRequired,
		type MfaRequirementImpact
	} from '#lib/mfaRequirement.js';
	import Text from '#lib/components/atoms/Text.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import SecondFactorNeeded from '#lib/components/molecules/SecondFactorNeeded.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import type { PracticeSession } from '../../+layout.js';

	// The Membership comes off practices/[practiceId]/+layout.ts's
	// already-resolved read (#835), not a fetch of this page's own.
	const session = $derived((page.data as { session: PracticeSession }).session);
	let isOwner = $derived(checkIsOwner(session));
	// #1532: turning the switch on is one of the five acts the BFF refuses
	// without a second factor of her own. `hasSecondFactor` (#lib/roles.ts)
	// holds the rule for a session that says nothing.
	const hasSecondFactor = $derived(checkHasSecondFactor(session));

	let impact = $state<MfaRequirementImpact | undefined>();
	let loadError = $state('');
	let toggleError = $state('');
	let successNotice = $state('');
	let isConfirmOpen = $state(false);
	let isSubmitting = $state(false);

	onMount(async () => {
		// The impact endpoint is Owner-only server-side (narrower than
		// payments' Owner-or-Admin status read, and than website's); asking
		// a non-Owner for it would only earn a 403 she has no use for, so
		// this never asks.
		if (isOwner) {
			await loadImpact();
		}
	});

	async function loadImpact() {
		loadError = '';
		try {
			impact = await loadMfaRequirementImpact(apiFetchWithSession, page.params.practiceId!);
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load this setting';
		}
	}

	// Common to both directions: the confirmation step before this runs is
	// the only thing that differs between them (#606).
	async function save(isRequired: boolean, confirmedMessage: string) {
		toggleError = '';
		successNotice = '';
		isSubmitting = true;
		try {
			await setMfaRequired(apiFetchWithSession, page.params.practiceId!, isRequired);
			successNotice = confirmedMessage;
			await loadImpact();
		} catch (error_) {
			toggleError = error_ instanceof Error ? error_.message : 'Failed to save this setting';
			throw error_;
		} finally {
			isSubmitting = false;
		}
	}

	function handleRequireAll() {
		isConfirmOpen = true;
	}

	// Rethrown by save() so ConfirmDialog stays open and renders toggleError
	// inside itself (#804).
	async function handleConfirmRequireAll() {
		await save(true, 'Every Staff member must now sign in with a second factor.');
	}

	async function handleStopRequiring() {
		try {
			await save(false, 'Staff without a second factor can sign in without one again.');
		} catch {
			// No ConfirmDialog gates this direction (#606) -- toggleError set
			// by save() above already renders on the page, so there is
			// nothing here that needs the rejection.
		}
	}

	// Singular/plural verbs, not just nouns ("has" vs "have"), spelled out
	// per branch rather than built from one suffix. Plain if/else, not a
	// nested ternary (unicorn/no-nested-ternary).
	function requireConsequence(count: number): string {
		if (count === 0) {
			return 'Every Staff member already has a second factor set up, so nobody will be signed out.';
		}
		if (count === 1) {
			return '1 Staff member has no second factor, and will not be able to sign in to this Practice until they set one up.';
		}
		return `${count} Staff members have no second factor, and will not be able to sign in to this Practice until they set one up.`;
	}

	let confirmConsequence = $derived(
		impact === undefined ? '' : requireConsequence(impact.withoutSecondFactor)
	);

	// Truthy until an Owner's impact has loaded -- a non-Owner's roles are
	// already known from page.data.session, so there is nothing further to
	// load for her.
	let loading = $derived(isOwner && impact === undefined ? 'Loading this setting' : undefined);
</script>

{#snippet intro()}
	<Text
		text="Turn this on to require a second factor for every Staff member at this Practice, Owners included."
	/>
{/snippet}

{#snippet body()}
	{#if !isOwner}
		<Notice variant="status" message="Only a Practice Owner can view or change this setting." />
	{:else}
		<!--
			A non-null assertion, not another `{#if impact}`: `body` only
			renders once `loading` is falsy, and `loading` stays truthy for
			an Owner until `impact` is set -- so `impact` is always defined
			by the time this runs, the same reasoning payments/+page.svelte's
			own `status!` uses.
		-->
		<!--
			"Mandatory", not "Required": formErrors.usage.spec.ts (#467)
			greps every quoted string in a component for GOV.UK's banned
			words, "required" among them, so this screen's own status word
			deliberately picks a synonym rather than becoming an inline
			exception to a rule the rest of the app holds absolutely.
		-->
		<!--
			A status is a row (#2046): the key and its badge on one line of a
			summary list, and the count a row of its own, where a label
			sentence with a colon and a pill beside it dropped the pill
			under its label at 320px.
		-->
		<SummaryList
			rows={[
				{ label: 'Second factor', value: impact!.required ? 'Mandatory for every Staff member' : 'Optional for every Staff member' },
				{ label: 'Staff without a second factor', value: String(impact!.withoutSecondFactor) }
			]}
		>
			{#snippet value(row, index)}
				{#if index === 0}
					<Badge label={row.value} variant={impact!.required ? 'success' : 'neutral'} />
				{:else}
					{row.value}
				{/if}
			{/snippet}
		</SummaryList>
		{#if !hasSecondFactor && !impact!.required}
			<SecondFactorNeeded
				message="You need two-factor authentication yourself before you can require it for every Staff member."
				returnTo={page.url.pathname}
			/>
		{/if}
		{#if successNotice}
			<Notice variant="status" message={successNotice} />
		{/if}
		{#if toggleError && !isConfirmOpen}
			<!-- Require-all's own failure renders inside ConfirmDialog while
			     it is open (#804); this is Stop-requiring's, which has no
			     dialog to gate it. -->
			<Notice variant="error" message={toggleError} />
		{/if}
	{/if}
{/snippet}

{#snippet actions()}
	{#if isOwner}
		{#if impact!.required}
			<Button
				label="Stop requiring MFA for all Staff"
				variant="secondary"
				loading={isSubmitting}
				onClick={handleStopRequiring}
			/>
		{:else if hasSecondFactor}
			<Button label="Require MFA for all Staff" loading={isSubmitting} onClick={handleRequireAll} />
		{/if}
	{/if}
{/snippet}

<FormPage title="Multi-factor authentication" {intro} fieldsets={[{ content: body }]} {actions} {loading} {loadError} />

<ConfirmDialog
	bind:open={isConfirmOpen}
	title="Require a second factor for every Staff member"
	consequence={confirmConsequence}
	confirmLabel="Require MFA for all Staff"
	error={toggleError}
	onConfirm={handleConfirmRequireAll}
/>

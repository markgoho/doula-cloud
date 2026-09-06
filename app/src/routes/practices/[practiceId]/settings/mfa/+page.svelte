<script lang="ts">
	/*
	 * Where an Owner raises MFA from "mandatory for Owners, optional for
	 * everyone else" to "mandatory for every Staff member" (#606).
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
	 * The GET this reads is Owner-only server-side, unlike payments' and
	 * website's own status endpoints, which any Staff member may read. So
	 * a non-Owner is never asked for it: she sees a notice and nothing
	 * about the setting itself, rather than a status everyone else's
	 * settings screens show with only the button removed.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { isOwner as checkIsOwner } from '#lib/roles.js';
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
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import type { PracticeSession } from '../../+layout.js';

	// The Membership comes off practices/[practiceId]/+layout.ts's
	// already-resolved read (#835), not a fetch of this page's own.
	const session = $derived((page.data as { session: PracticeSession }).session);
	let isOwner = $derived(checkIsOwner(session));

	let impact = $state<MfaRequirementImpact | undefined>();
	let loadError = $state('');
	let toggleError = $state('');
	let successNotice = $state('');
	let isConfirmOpen = $state(false);
	let isSubmitting = $state(false);

	onMount(async () => {
		// The impact endpoint is Owner-only server-side (unlike payments'
		// and website's own status reads); asking a non-Owner for it would
		// only earn a 403 she has no use for, so this never asks.
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
		} finally {
			isSubmitting = false;
		}
	}

	function handleRequireAll() {
		isConfirmOpen = true;
	}

	async function handleConfirmRequireAll() {
		await save(true, 'Every Staff member must now sign in with a second factor.');
	}

	async function handleStopRequiring() {
		await save(false, 'Staff without a second factor can sign in without one again.');
	}

	// Singular/plural verbs, not just nouns ("has" vs "have") -- the same
	// distinction payments/+page.svelte's "Stripe needs 1 more detail" vs
	// "N more details" makes, but this sentence also conjugates the verb,
	// so it is spelled out per branch rather than built from one suffix.
	// Plain if/else, not a nested ternary (unicorn/no-nested-ternary), for
	// both this and requireConsequence below.
	function staffCountSentence(count: number): string {
		if (count === 1) return '1 Staff member currently has no second factor set up.';
		return `${count} Staff members currently have no second factor set up.`;
	}

	function requireConsequence(count: number): string {
		if (count === 0) {
			return 'Every Staff member already has a second factor set up, so nobody will be signed out.';
		}
		if (count === 1) {
			return '1 Staff member has no second factor, and will not be able to sign in to this Practice until they set one up.';
		}
		return `${count} Staff members have no second factor, and will not be able to sign in to this Practice until they set one up.`;
	}

	let staffCountText = $derived(impact === undefined ? '' : staffCountSentence(impact.withoutSecondFactor));

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
		text="Every Owner must already sign in with a second factor. Turn this on to require it for every Staff member at this Practice."
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
		<cluster-l>
			<Text text="Second factor:" />
			<Badge
				label={impact!.required
					? 'Mandatory for every Staff member'
					: 'Mandatory for Owners, optional for other Staff'}
				variant={impact!.required ? 'success' : 'neutral'}
			/>
		</cluster-l>
		<Text text={staffCountText} />
		{#if successNotice}
			<Notice variant="status" message={successNotice} />
		{/if}
		{#if toggleError}
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
		{:else}
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
	onConfirm={handleConfirmRequireAll}
/>

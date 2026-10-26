<script lang="ts">
	/*
	 * Settings (#431, #2028): every setting a person may change, in four
	 * headed groups, each a `SummaryList` row with its current value and a
	 * Change. GOV.UK's Summary list is the shape a settings page takes, and
	 * the groups are Miller's Law: thirteen settings in one run had no group
	 * at all, and showed no value, so the timezone or the rates took a page
	 * each to read.
	 *
	 * A row is drawn only for someone who may change it (`MAY_CHANGE`,
	 * which names each write's role list): a Doula's Settings is the one
	 * setting the BFF lets her write. The values come off the same loaders
	 * each setting's screen reads (`loadSettingsSummary`), side by side.
	 *
	 * Deletion is last and on its own, under its own heading: it is the one
	 * act here that cannot be put back, and it must not read like "Rates".
	 */
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiBaseURL, apiFetchWithSession } from '#lib/api.js';
	import { hasSecondFactor as checkHasSecondFactor } from '#lib/roles.js';
	import { loadSettingsSummary, MAY_CHANGE, type SettingKey, type SettingsSummary } from '#lib/settingsSummary.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import SecondFactorNeeded from '#lib/components/molecules/SecondFactorNeeded.svelte';
	import SummaryList, { type SummaryListRow } from '#lib/components/molecules/SummaryList.svelte';
	import OverviewHub from '#lib/components/templates/OverviewHub.svelte';
	import type { PracticeSession } from '../+layout.js';

	const practiceId = $derived(page.params.practiceId!);
	// The Membership comes off practices/[practiceId]/+layout.ts's
	// already-resolved read (#835), not a fetch of this page's own.
	const session = $derived((page.data as { session: PracticeSession }).session);
	// #1532: downloading the archive is one of the five acts the BFF
	// refuses without a second factor, so the row says what it needs in
	// place of a download link that would land her on the BFF's JSON.
	const hasSecondFactor = $derived(checkHasSecondFactor(session));

	let summary = $state<SettingsSummary | undefined>();

	onMount(async () => {
		summary = await loadSettingsSummary(apiFetchWithSession, practiceId, session);
	});

	interface SettingRow extends SummaryListRow {
		key: SettingKey;
		href: string;
		/**
		The action's word, where it is not "Change".
		*/
		act?: string;
	}

	interface SettingGroup {
		heading: string;
		rows: SettingRow[];
	}

	const groups: SettingGroup[] = $derived(
		(
		[
			{
				heading: 'Your Practice',
				rows: [
					{ key: 'name', label: 'Practice name', value: session.practiceName, changes: 'the Practice name', href: resolve('/practices/[practiceId]/settings/name', { practiceId }) },
					{ key: 'website', label: 'Your website', value: '', changes: 'your website', href: resolve('/practices/[practiceId]/settings/website', { practiceId }) },
					{ key: 'timezone', label: 'Timezone', value: '', changes: 'the timezone', href: resolve('/practices/[practiceId]/settings/timezone', { practiceId }) }
				]
			},
			{
				heading: 'Clients and care',
				rows: [
					{ key: 'clientFields', label: 'Client Fields', value: '', changes: 'the Client Fields', href: resolve('/practices/[practiceId]/settings/client-fields', { practiceId }) },
					{ key: 'planTemplates', label: 'Plan Templates', value: '', changes: 'the Plan Templates', href: resolve('/practices/[practiceId]/settings/plan-templates', { practiceId }) },
					{ key: 'contractTemplate', label: 'Contract Template', value: '', changes: 'the Contract Template', href: resolve('/practices/[practiceId]/settings/contract-template', { practiceId }) },
					{ key: 'onCall', label: 'On call', value: '', changes: 'when on call starts and ends', href: resolve('/practices/[practiceId]/settings/on-call', { practiceId }) }
				]
			},
			{
				heading: 'Money',
				rows: [
					// Getting paid, not Payments (#256): the route stays
					// `/settings/payments` (ADR-0032), but a Payment is money
					// received against an Invoice, which this screen holds none of.
					{ key: 'gettingPaid', label: 'Getting paid', value: '', changes: 'how this Practice gets paid', href: resolve('/practices/[practiceId]/settings/payments', { practiceId }) },
					{ key: 'rates', label: 'Rates', value: '', changes: 'the rates', href: resolve('/practices/[practiceId]/settings/rates', { practiceId }) },
					{ key: 'credits', label: 'Credits', value: '', changes: 'credits', act: 'Buy', href: resolve('/practices/[practiceId]/billing', { practiceId }) }
				]
			},
			{
				heading: 'Security and data',
				rows: [
					{ key: 'mfa', label: 'Multi-factor authentication', value: '', changes: 'multi-factor authentication', href: resolve('/practices/[practiceId]/settings/mfa', { practiceId }) },
					{ key: 'blockedAddresses', label: 'Blocked email addresses', value: '', changes: 'the blocked email addresses', href: resolve('/practices/[practiceId]/settings/blocked-addresses', { practiceId }) },
					// #288: a download the browser fetches on its own, so the BFF's
					// own address rather than resolve(). Without a second factor
					// the row has no action: its value says what the act needs.
					{
						key: 'export',
						label: "Export this Practice's data",
						value: 'Every record, as one ZIP of spreadsheet-ready files',
						...(hasSecondFactor && { changes: "this Practice's data" }),
						act: 'Download',
						href: `${apiBaseURL()}/api/practices/${practiceId}/export`
					}
				]
			}
		] satisfies SettingGroup[]
		)
			.map((group) => ({ ...group, rows: group.rows.filter((row) => MAY_CHANGE[row.key](session)) }))
			.filter((group) => group.rows.length > 0)
	);

	const isMayDelete = $derived(MAY_CHANGE.delete(session));
</script>

{#snippet value(row: SettingRow)}
	{@const read = summary?.[row.key]}
	{#if row.key === 'export' && !hasSecondFactor}
		<SecondFactorNeeded
			message="You need two-factor authentication before you can export this Practice's data."
			returnTo={page.url.pathname}
		/>
	{:else if row.value !== ''}
		{row.value}
	{:else if read === undefined}
		<Skeleton variant="text" lines={1} label="Loading {row.label}" />
	{:else if read === 'unavailable'}
		Could not load this. Open it to see why.
	{:else if read.href}
		<Link href={read.href} label={read.text} />
	{:else}
		{read.text}
	{/if}
{/snippet}

{#snippet action(row: SettingRow, describedBy: string)}
	<Link href={row.href} label={row.act ?? 'Change'} {describedBy} />
{/snippet}

{#snippet primary()}
	{#each groups as group (group.heading)}
		<section>
			<stack-l space="var(--space-related)">
				<Heading level={2} variant="section" text={group.heading} />
				<SummaryList rows={group.rows} {value} {action} />
			</stack-l>
		</section>
	{/each}
	{#if isMayDelete}
		<section class="apart">
			<stack-l space="var(--space-related)">
				<Heading level={2} variant="section" text="Delete this Practice" />
				<Text text="This starts a 30-day countdown. When it ends, every Client is erased." />
				<Link href={resolve('/practices/[practiceId]/settings/delete', { practiceId })} label="Delete this Practice" />
			</stack-l>
		</section>
	{/if}
{/snippet}

{#snippet empty()}
	<!--
		Unreachable in practice: every Staff member may change the Plan
		Templates, so no session has an empty Settings. The region is
		required by OverviewHub on purpose (#422).
	-->
	<Text text="This Practice has no settings you can change." />
{/snippet}

<OverviewHub title="Settings" isEmpty={false} {primary} {empty} />

<style>
	@layer components {
		/* Set apart: a rule above it, and a section's gap over the group's. */
		.apart {
			/* The summary lists' own measure, so the rule ends where they do. */
			max-inline-size: var(--measure);
			margin-block-start: var(--space-group);
			padding-block-start: var(--space-group);
			border-block-start: var(--border-thin) solid var(--color-outline-variant);
		}
	}
</style>

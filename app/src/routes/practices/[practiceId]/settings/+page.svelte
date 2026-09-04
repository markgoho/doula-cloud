<script lang="ts">
	/*
	 * Settings is a nav item in the shell (#431), so it needs somewhere to
	 * land. Before this ticket there was no `/settings` page at all: the
	 * four settings screens were reachable only from the temporary header of
	 * links the shell replaces, so Plan Templates, Contract Template and
	 * Website would have been orphaned the moment that header went.
	 *
	 * Archetype F is outside this map's destination, so this is deliberately
	 * the smallest thing that keeps every screen reachable -- a way in, not
	 * a settings design. What each screen is called and how the group is
	 * ordered belongs to whoever builds archetype F.
	 */
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import Link from '#lib/components/atoms/Link.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import OverviewHub from '#lib/components/templates/OverviewHub.svelte';

	const practiceId = $derived(page.params.practiceId!);

	const settings = $derived([
		{
			label: 'Payments',
			description: 'How this Practice gets paid, and what Stripe still wants from it.',
			href: resolve('/practices/[practiceId]/settings/payments', { practiceId })
		},
		{
			label: 'Website',
			description: 'The address a Client arrives from.',
			href: resolve('/practices/[practiceId]/settings/website', { practiceId })
		},
		{
			label: 'Client Fields',
			description: 'The extra questions this Practice asks every Client, beyond the standard ones.',
			href: resolve('/practices/[practiceId]/settings/client-fields', { practiceId })
		},
		{
			label: 'Plan Templates',
			description: 'The questions every birth plan and care plan starts from.',
			href: resolve('/practices/[practiceId]/settings/plan-templates', { practiceId })
		},
		{
			label: 'Contract Template',
			description: 'The terms every Contract is written from.',
			href: resolve('/practices/[practiceId]/settings/contract-template', { practiceId })
		}
	]);
</script>

{#snippet primary()}
	<ul>
		{#each settings as setting (setting.href)}
			<li>
				<Link href={setting.href} label={setting.label} />
				<Text text={setting.description} step="body-sm" tone="variant" />
			</li>
		{/each}
	</ul>
{/snippet}

{#snippet empty()}
	<!--
		Unreachable in practice: the list is a fixed four, not a query. The
		region is required by OverviewHub on purpose (#422), and a Template
		that cannot be instantiated without an empty state is what makes the
		first-run case impossible to forget.
	-->
	<Text text="This Practice has no settings screens." />
{/snippet}

<OverviewHub title="Settings" isEmpty={false} {primary} {empty} />

<style>
	@layer components {
		ul {
			display: flex;
			flex-direction: column;
			gap: var(--space-6);
			margin: 0;
			padding: 0;
			list-style: none;
		}

		li {
			display: flex;
			flex-direction: column;
			gap: var(--space-1);
		}
	}
</style>

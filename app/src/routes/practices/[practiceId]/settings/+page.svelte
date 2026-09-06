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
	import { isOwner, isOwnerOrAdmin } from '#lib/roles.js';
	import Link from '#lib/components/atoms/Link.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import OverviewHub from '#lib/components/templates/OverviewHub.svelte';
	import type { PracticeSession } from '../+layout.js';

	const practiceId = $derived(page.params.practiceId!);

	// Multi-factor authentication (#606) is the first Owner-only entry
	// this hub has ever needed to hide: its screen reads an Owner-only
	// endpoint, so a Doula or Admin who followed the link would only meet
	// a 403. The Membership comes off practices/[practiceId]/+layout.ts's
	// already-resolved read (#835), not a fetch of this page's own.
	const session = $derived((page.data as { session: PracticeSession }).session);
	let isPracticeOwner = $derived(isOwner(session));
	// Blocked email addresses (#744) is the second gated entry, and gated
	// one notch wider: its list is every Client and Staff address at the
	// Practice whose mail is failing, which ADR-0008 keeps in the same
	// hands as the roster it is drawn from -- Owner or Admin, matching the
	// endpoint's own `ownerAndAdmin` guard.
	let isPracticeOwnerOrAdmin = $derived(isOwnerOrAdmin(session));

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
		},
		...(isPracticeOwnerOrAdmin
			? [
					{
						label: 'Blocked email addresses',
						description:
							'The addresses Doula Cloud has stopped writing to, and why each one stopped.',
						href: resolve('/practices/[practiceId]/settings/blocked-addresses', { practiceId })
					}
				]
			: []),
		...(isPracticeOwner
			? [
					{
						label: 'Multi-factor authentication',
						description: 'Whether every Staff member must use a second factor to sign in, not only Owners.',
						href: resolve('/practices/[practiceId]/settings/mfa', { practiceId })
					}
				]
			: [])
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
		Unreachable in practice: the list is fixed content gated only by
		role (#606), not a query. The region is required by OverviewHub on
		purpose (#422), and a Template that cannot be instantiated without
		an empty state is what makes the first-run case impossible to
		forget.
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

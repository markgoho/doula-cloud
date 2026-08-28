<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import Link from '#lib/components/atoms/Link.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import type { IconName } from '#lib/components/atoms/Icon/manifest.js';
	import PenTrial411ActivityRow from './PenTrial411ActivityRow.svelte';

	// Trial page for the pen.dev "Staff Dashboard" design. Everything below is
	// the design's own fixture copy -- nothing is fetched, so none of it needs
	// to be reactive state.
	const practiceName = 'Riverside Doula Collective';
	const practiceId = $derived(page.params.practiceId ?? '');

	const navItems = $derived([
		{ label: 'Overview', href: resolve('/practices/[practiceId]', { practiceId }) },
		{ label: 'Clients', href: resolve('/practices/[practiceId]/clients', { practiceId }) },
		{ label: 'Billing', href: resolve('/practices/[practiceId]/billing', { practiceId }) },
		{ label: 'Staff', href: resolve('/practices/[practiceId]/staff', { practiceId }) },
		{ label: 'Offers', href: resolve('/practices/[practiceId]/offers', { practiceId }) }
	]);

	const quickLinks: { label: string; icon: IconName; href: string }[] = $derived([
		{
			label: 'Billing',
			icon: 'receipt',
			href: resolve('/practices/[practiceId]/billing', { practiceId })
		},
		{
			label: 'Your offers',
			icon: 'tag',
			href: resolve('/practices/[practiceId]/offers', { practiceId })
		},
		{
			label: 'Staff',
			icon: 'user-check',
			href: resolve('/practices/[practiceId]/staff', { practiceId })
		},
		{
			label: 'Plan Templates',
			icon: 'clipboard-text',
			href: resolve('/practices/[practiceId]/settings/plan-templates', { practiceId })
		},
		{
			label: 'Contract Template',
			icon: 'file-text',
			href: resolve('/practices/[practiceId]/settings/contract-template', { practiceId })
		},
		{
			label: 'Payments',
			icon: 'credit-card',
			href: resolve('/practices/[practiceId]/settings/payments', { practiceId })
		},
		{
			label: 'Clients',
			icon: 'users',
			href: resolve('/practices/[practiceId]/clients', { practiceId })
		}
	]);

	const activity = [
		{
			timestamp: '2026-08-28 09:41',
			description: 'Invoice INV-2041 sent to Amara Okafor for $1,850.00',
			actor: 'Mark Goho'
		},
		{
			timestamp: '2026-08-28 08:57',
			description: 'Danielle Ruiz accepted the invitation to join the practice',
			actor: 'System'
		},
		{
			timestamp: '2026-08-27 17:22',
			description: 'Employment type changed for Priya Raman: contractor to employee',
			actor: 'Mark Goho'
		},
		{
			timestamp: '2026-08-27 15:05',
			description: 'Plan “Full Spectrum Birth + 4 Postpartum Visits” assigned to Jordan Wells',
			actor: 'Tasha Lin'
		},
		{
			timestamp: '2026-08-27 11:38',
			description: 'Payment of $600.00 received against invoice INV-2038',
			actor: 'Stripe'
		},
		{
			timestamp: '2026-08-26 16:12',
			description: 'Contract template “Standard Birth Doula Agreement” updated to v4',
			actor: 'Mark Goho'
		},
		{
			timestamp: '2026-08-26 10:04',
			description: 'Client record created for Simone Adeyemi, due 2026-11-14',
			actor: 'Tasha Lin'
		},
		{
			timestamp: '2026-08-25 14:47',
			description: 'Offer “Postpartum Support Package” archived',
			actor: 'Danielle Ruiz'
		}
	];
</script>

<div class="dashboard">
	<header class="top-bar">
		<div class="brand-and-nav">
			<span class="wordmark">Doula Cloud</span>
			<nav aria-label="Practice">
				<ul>
					{#each navItems as item (item.label)}
						<li>
							<Link href={item.href} label={item.label} current={item.label === 'Overview'} />
						</li>
					{/each}
				</ul>
			</nav>
		</div>

		<div class="account">
			<Button variant="secondary" size="sm" icon="caret-down" label={practiceName} />
			<span class="avatar">MG</span>
		</div>
	</header>

	<main class="page-content">
		<h1>Welcome to {practiceName}</h1>

		<ul class="quick-links">
			{#each quickLinks as link (link.label)}
				<li>
					<Link variant="card" href={link.href} label={link.label} icon={link.icon} />
				</li>
			{/each}
		</ul>

		<section class="recent-activity" aria-labelledby="recent-activity-title">
			<div class="panel-header">
				<h2 id="recent-activity-title">Recent activity</h2>
				<p class="panel-meta">Last 4 days · newest first</p>
			</div>
			<ol class="activity-list">
				{#each activity as entry (entry.timestamp)}
					<PenTrial411ActivityRow
						timestamp={entry.timestamp}
						description={entry.description}
						actor={entry.actor}
					/>
				{/each}
			</ol>
		</section>
	</main>
</div>

<style>
	@layer components {
		/* tokens.css has no --color-surface / --color-surface-muted yet. The
		   design's white chrome and its pale plum panel are asked for by those
		   names with light fallbacks, so both pick the tokens up unchanged once
		   they land. --color-panel-bg is deliberately not reused: it is the dark
		   plum panel, not this design's light one. */
		.dashboard {
			display: flex;
			flex-direction: column;
			min-block-size: 100vb;
			background-color: var(--color-bg);
			font-family: var(--font-family-base);
		}

		/* Top bar */

		.top-bar {
			display: flex;
			justify-content: space-between;
			align-items: center;
			block-size: 60px;
			padding-inline: var(--space-10, 2.5rem);
			border-block-end: var(--border-thin) solid var(--color-border);
			background-color: var(--color-surface, oklch(99% 0.004 320));
		}

		.brand-and-nav {
			display: flex;
			align-items: center;
			gap: 44px;
			align-self: stretch;
		}

		.wordmark {
			color: var(--color-text);
			font-size: var(--text-lg);
			font-weight: var(--font-weight-semibold);
			letter-spacing: -0.2px;
		}

		nav,
		nav ul {
			display: flex;
			align-self: stretch;
		}

		nav ul {
			gap: 2px;
			margin: 0;
			padding: 0;
			list-style: none;
		}

		nav li {
			display: flex;
		}

		nav li :global(a) {
			padding-inline: 14px;
			block-size: 100%;
		}

		.account {
			display: flex;
			align-items: center;
			gap: 18px;
		}

		.avatar {
			display: flex;
			justify-content: center;
			align-items: center;
			inline-size: 34px;
			block-size: 34px;
			border: var(--border-thin) solid var(--color-border);
			border-radius: 50%;
			background-color: var(--color-surface-muted, oklch(96% 0.009 350));
			color: var(--color-text);
			font-size: 0.8125rem;
			font-weight: var(--font-weight-semibold);
			letter-spacing: 0.3px;
		}

		/* Page content */

		.page-content {
			display: flex;
			flex-direction: column;
			gap: 34px;
			padding: 38px var(--space-10, 2.5rem) var(--space-12) var(--space-10, 2.5rem);
		}

		h1 {
			margin: 0;
			color: var(--color-text);
			font-size: var(--text-3xl);
			font-weight: 700;
			line-height: 1.2;
			letter-spacing: -0.7px;
		}

		/* Quick links */

		.quick-links {
			display: flex;
			flex-wrap: wrap;
			gap: var(--space-5, 1.25rem);
			margin: 0;
			padding: 0;
			list-style: none;
		}

		/* The design puts all seven cards on one row of a 1440px frame; the
		   basis keeps that at full width and lets them wrap rather than crush
		   below it. */
		.quick-links li {
			display: flex;
			flex: 1 1 9rem;
		}

		/* Recent activity */

		.recent-activity {
			overflow: hidden;
			border: var(--border-thin) solid var(--color-border);
			border-radius: var(--radius-sm);
			background-color: var(--color-surface-muted, oklch(96% 0.009 350));
		}

		.panel-header {
			display: flex;
			justify-content: space-between;
			align-items: center;
			padding: 17px var(--space-6);
			border-block-end: var(--border-thin) solid var(--color-border);
		}

		.panel-header h2 {
			margin: 0;
			color: var(--color-text);
			font-size: var(--text-lg);
			font-weight: var(--font-weight-semibold);
		}

		.panel-meta {
			margin: 0;
			color: var(--color-muted);
			font-size: 0.8125rem;
			font-weight: var(--font-weight-normal);
		}

		.activity-list {
			margin: 0;
			padding: 2px var(--space-6) var(--space-2) var(--space-6);
			list-style: none;
		}
	}
</style>

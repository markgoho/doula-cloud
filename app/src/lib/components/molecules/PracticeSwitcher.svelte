<script module lang="ts">
	/*
	 * One Practice a person can be looking at, already resolved to a route
	 * by the shell. The switcher itself knows nothing about routing -- it is
	 * handed hrefs the same way SignOutButton is handed a `signOut`.
	 */
	export interface PracticeOption {
		practiceId: string;
		practiceName: string;
		roles: string[];
		href: string;
	}

	// ADR-0008's three roles, title-cased for reading. A role the BFF grows
	// later and this map has not caught up with still prints, capitalised,
	// rather than disappearing from a person's own list of what she is.
	const ROLE_LABELS: Record<string, string> = {
		owner: 'Owner',
		admin: 'Admin',
		doula: 'Doula'
	};

	export function rolesLabel(roles: string[]): string {
		return roles
			.map((role) => ROLE_LABELS[role] ?? role.charAt(0).toLocaleUpperCase() + role.slice(1))
			.join(', ');
	}
</script>

<script lang="ts">
	import Icon from '#lib/components/atoms/Icon.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import MenuButton from './MenuButton.svelte';

	/*
	 * Which Practice the person is looking at, and the way to the others
	 * (#431). A contractor Doula holds Memberships at several agencies --
	 * the pilot has one working at three -- so this is not an edge case
	 * bolted onto a single-Practice product.
	 */
	interface Properties {
		practices: PracticeOption[];
		currentPracticeId: string;
	}

	let { practices, currentPracticeId }: Properties = $props();

	const current = $derived(practices.find((practice) => practice.practiceId === currentPracticeId));
	// One Membership is still worth naming -- a person should be able to see
	// which Practice she is in -- but a control that opens a list of one is
	// a promise the product cannot keep, so the caret and the panel appear
	// only at two or more.
	const isSwitchable = $derived(practices.length > 1);
</script>

{#if current}
	{#if isSwitchable}
		<MenuButton label={current.practiceName} icon="caret-down" iconPosition="end" align="end">
			<p class="heading">Your practices</p>
			<ul>
				{#each practices as practice (practice.practiceId)}
					{@const isCurrent = practice.practiceId === currentPracticeId}
					<li class:current={isCurrent}>
						<Link
							href={practice.href}
							label={practice.practiceName}
							variant="secondary"
							current={isCurrent}
						/>
						<span class="roles">{rolesLabel(practice.roles)}</span>
						{#if isCurrent}
							<span class="tick"><Icon name="check" size={16} label="Current practice" /></span>
						{/if}
					</li>
				{/each}
			</ul>
		</MenuButton>
	{:else}
		<span class="single">{current.practiceName}</span>
	{/if}
{/if}

<style>
	@layer components {
		.heading {
			margin: 0;
			padding: var(--space-2) var(--space-4) var(--space-1);
			color: var(--color-on-surface-muted);
			font-size: var(--text-meta-size);
			letter-spacing: var(--text-meta-tracking);
			text-transform: uppercase;
		}

		ul {
			margin: 0;
			padding: 0;
			list-style: none;
		}

		li {
			display: grid;
			grid-template-columns: 1fr auto;
			align-items: center;
			gap: 0 var(--space-3);
			min-block-size: var(--nav-row-height);
			padding: var(--space-2) var(--space-4);
		}

		li.current {
			background-color: var(--color-surface-container);
		}

		.roles {
			grid-column: 1;
			color: var(--color-on-surface-muted);
			font-size: var(--text-meta-size);
		}

		.tick {
			grid-row: 1 / span 2;
			grid-column: 2;
			color: var(--color-primary);
		}

		/* The name alone when there is nothing to switch to. Same weight and
		   size as the switcher's own label, so a person who joins a second
		   Practice sees a caret appear rather than the bar re-typesetting. */
		.single {
			display: inline-flex;
			align-items: center;
			min-block-size: var(--hit-target-min);
			padding-inline: var(--space-2);
			color: var(--color-on-surface);
			font-family: var(--font-family-base);
			font-size: var(--text-body-sm-size);
		}
	}
</style>

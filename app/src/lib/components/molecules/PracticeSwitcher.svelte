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

	/*
	 * The Practice a person is looking at, or nothing at all -- a route
	 * scoped to the person rather than to a Practice (/account, #484) hands
	 * this an empty list and an empty id, and so does any Practice-scoped
	 * route in the moment before the Memberships arrive.
	 *
	 * Exported because it is the switcher's own render condition, and a
	 * caller that frames the switcher -- the Staff bar's sheet puts a
	 * heading and a divider above it (#673) -- has to know whether there
	 * will be anything inside the frame. Restating the predicate at the
	 * caller is the same fact written twice, and the copy is what goes
	 * stale. Same reason `initialsOf` is exported from Avatar.
	 */
	export function currentPracticeOf(
		practices: PracticeOption[],
		currentPracticeId: string
	): PracticeOption | undefined {
		return practices.find((practice) => practice.practiceId === currentPracticeId);
	}
</script>

<script lang="ts">
	import DoorLink from '#lib/components/atoms/DoorLink.svelte';
	import MenuButton from './MenuButton.svelte';
	import { rolesLabel } from '#lib/roles.js';

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

	const current = $derived(currentPracticeOf(practices, currentPracticeId));
	// One Membership is still worth naming -- a person should be able to see
	// which Practice she is in -- but a control that opens a list of one is
	// a promise the product cannot keep, so the caret and the panel appear
	// only at two or more.
	const isSwitchable = $derived(practices.length > 1);

	// The Practice she is in first, then the others in the order they
	// arrive (#2141): the top of the list answers "where am I" before it
	// offers anywhere else, as the drawing had it.
	function currentFirst(currentPractice: PracticeOption): PracticeOption[] {
		return [currentPractice, ...practices.filter((practice) => practice !== currentPractice)];
	}
</script>

{#if current}
	{#if isSwitchable}
		<MenuButton label={current.practiceName} icon="caret-down" iconPosition="end" align="end">
			<p class="heading">Your practices</p>
			<!-- No preload (#2070): a hover would load that Practice's session,
			     which the BFF records as her last-used Practice. -->
			<ul data-sveltekit-preload-data="false">
				{#each currentFirst(current) as practice (practice.practiceId)}
					<li>
						<DoorLink
							href={practice.href}
							label={practice.practiceName}
							description={rolesLabel(practice.roles)}
							variant="menu"
							current={practice === current}
						/>
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
		}

		ul {
			margin: 0;
			padding: 0;
			list-style: none;
		}

		li {
			display: flex;
			flex-direction: column;
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

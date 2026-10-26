<script lang="ts">
	/*
	 * Choosing which Practice to open (#2027): a stack of doors, one per
	 * Practice, its name as the link and the roles held there as the line
	 * under it. Every account switcher in the #406 survey is a list of
	 * cards or rows (Jakob's Law), and the signed-out landing beside it
	 * already draws its doors as `DoorLink`s.
	 *
	 * Two consumers that should change together (ADR-0018's 2026-10-25
	 * bar): `/` for a Staff member with several Practices, and `/login`
	 * once her password is accepted. Not `PracticeSwitcher`: that is the
	 * bar's menu for moving between Practices once one is open, and this
	 * is the page a person lands on when none is open yet.
	 *
	 * Handed each Practice's href already resolved, as `PracticeSwitcher`
	 * is, so it knows nothing about routing. A `<ul>`, so a screen reader
	 * hears how many Practices there are; the reset takes its markers away
	 * (#2017). A Membership always carries at least one role (GLOSSARY.md),
	 * so the line under each door is never empty.
	 */
	import DoorLink from '#lib/components/atoms/DoorLink.svelte';
	import { rolesLabel } from '#lib/roles.js';
	import type { PracticeOption } from './PracticeSwitcher.svelte';

	interface Properties {
		practices: PracticeOption[];
	}

	let { practices }: Properties = $props();
</script>

<!--
	data-sveltekit-preload-data="false" (#2070). app.html preloads a link's
	data on hover, and a door's load reads that Practice's session -- the
	request the BFF records as her last-used Practice. A pointer resting on
	a door is not a choice, so no door here may load ahead of the click.
-->
<ul data-sveltekit-preload-data="false">
	{#each practices as practice (practice.practiceId)}
		<li>
			<DoorLink href={practice.href} label={practice.practiceName} description={rolesLabel(practice.roles)} />
		</li>
	{/each}
</ul>

<style>
	@layer components {
		/* One group of related choices (#2020). */
		ul {
			display: flex;
			flex-direction: column;
			gap: var(--space-related);
			margin: 0;
		}

		li {
			display: flex;
			flex-direction: column;
		}
	}
</style>

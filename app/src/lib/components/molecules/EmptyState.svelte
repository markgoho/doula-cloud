<script module lang="ts">
	/**
	 * The one action that makes the first thing (#2015): a destination is a
	 * `Link`, a change on this same screen is a `Button`. Its shape picks
	 * which, so a caller never chooses an element, only what it does.
	 */
	export type EmptyStateAction = { label: string; href: string } | { label: string; onClick: () => void };
</script>

<script lang="ts">
	/*
	 * What an empty list or section shows (#2015, the #2001 audit's root
	 * cause 3). Before it, an empty list took one of four shapes: a table
	 * header over a one-cell row, a bare sentence, the same fact said twice,
	 * or a column of zeros. This is the one shape: the duotone icon the
	 * brief reserves for empty states, one plain line that says what is not
	 * here yet (Voice rule 5), and, where there is one, the one action that
	 * makes the first thing.
	 *
	 * Named by structure, not by look (ADR-0018). GOV.UK, NHS.UK and ONS
	 * have no empty-state component, so this is rule-5 original work; see
	 * its row in docs/design/govuk-alignment.md.
	 *
	 * One icon for every empty state, not one per list: the icon says
	 * "nothing here", and the line says what. A per-list icon would be a
	 * second way to say the line, and a choice every caller had to make.
	 * It is decoration, hidden from assistive technology, so the line is
	 * the whole of what a screen reader hears.
	 */
	import Button from '#lib/components/atoms/Button.svelte';
	import Icon from '#lib/components/atoms/Icon.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	interface Properties {
		message: string;
		action?: EmptyStateAction;
	}

	let { message, action }: Properties = $props();
</script>

<div class="empty-state">
	<span class="icon"><Icon name="tray" size={32} weight="duotone" /></span>
	<stack-l space="var(--space-3)">
		<Text text={message} measure />
		{#if action && 'href' in action}
			<div><Link href={action.href} label={action.label} /></div>
		{:else if action}
			<div><Button label={action.label} variant="secondary" onClick={action.onClick} /></div>
		{/if}
	</stack-l>
</div>

<style>
	@layer components {
		/* The icon beside the line, wrapping under it only when the line
		   would be narrower than a few words: flex-wrap with a basis on the
		   text, no width query (ADR-0024). */
		.empty-state {
			display: flex;
			flex-wrap: wrap;
			align-items: start;
			gap: var(--space-3) var(--space-4);
		}

		.empty-state > stack-l {
			flex: 1 1 12rem;
			min-inline-size: 0;
		}

		/* The subject of the screen, so it takes the accent (brief:
		   Iconography); the duotone's own 20% layer is the quiet half. */
		.icon {
			display: inline-flex;
			color: var(--color-primary);
		}
	}
</style>

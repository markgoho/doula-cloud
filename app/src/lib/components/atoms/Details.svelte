<script lang="ts">
	/**
	 * The GOV.UK Details component
	 * (https://design-system.service.gov.uk/components/details/) on a
	 * native `<details>`/`<summary>` -- closed unless the caller opens it,
	 * with the caller's summary text and body content. The platform's own
	 * keyboard handling, focus and the expanded/collapsed announcement do
	 * the work; nothing here adds an ARIA attribute on top of the element.
	 *
	 * `organisms/DataTable.svelte`'s `disclosure` prop and
	 * `molecules/HistoryDisclosure.svelte` each hand-wrote this shape
	 * before this atom existed (#1520); both now render through it, which
	 * is why the summary's sizing, the closed-content-hiding fix and the
	 * WCAG 2.2 target-size fix live here once instead of twice.
	 * `organisms/StepRail.svelte` keeps its own `<details>` -- see the
	 * comment at its own element for why.
	 */
	import { untrack, type Snippet } from 'svelte';

	interface Properties {
		/**
		 * The summary's visible text.
		 */
		summary: string;
		/**
		 * Text appended to the summary's accessible name only, hidden from
		 * sighted readers. `HistoryDisclosure`'s own need (#667): the Staff
		 * roster repeats the same summary on every row, and a screen
		 * reader's list of controls cannot tell them apart without this.
		 */
		hiddenSummary?: string;
		/**
		 * Whether the disclosure starts open. There is no bindable
		 * counterpart: a native `<details>` already tracks its own open
		 * state once the reader has clicked it, and a second source of
		 * truth would only disagree with the one the reader just chose.
		 */
		open?: boolean;
		/**
		 * Called after every toggle, open and closed alike, with the
		 * element's own `open` state.
		 */
		onToggle?: (isOpen: boolean) => void;
		/**
		 * The content the summary discloses.
		 */
		children: Snippet;
	}

	let { summary, hiddenSummary = '', open = false, onToggle, children }: Properties = $props();

	/*
	 * `open` seeds this once; nothing keeps writing it back from the prop.
	 * A caller such as `DataTable`/`HistoryDisclosure` rerenders on
	 * unrelated state (a page arriving, a row's own data changing), and
	 * `{open}` bound straight to the prop would reassert on every one of
	 * those passes -- closing a disclosure the reader had just opened,
	 * because the prop itself never changed away from its own default.
	 * Measured directly: a plain `<details {open}>` closed a user-opened
	 * disclosure the moment its parent rerendered for any other reason.
	 * `isOpen` instead tracks the element's OWN state from here on --
	 * `handleToggle` below is what keeps it truthful.
	 */
	let isOpen = $state(untrack(() => open));

	function handleToggle(event: Event & { currentTarget: HTMLDetailsElement }) {
		isOpen = event.currentTarget.open;
		onToggle?.(isOpen);
	}
</script>

<details open={isOpen} ontoggle={handleToggle}>
	<!--
		{' '} rather than a raw space: a raw space next to a block edge, or
		at the start of an element's own content, is template whitespace,
		and Svelte's whitespace trimming removes it either way -- tried
		both, both collapsed to "Membership historyfor Renata Alvarez" as
		one run-on word. A mustache-held string is content, not template
		whitespace, so it survives.
	-->
	<summary>{summary}{#if hiddenSummary}<!-- eslint-disable-line svelte/no-useless-mustaches -- {' '} is a literal space, not a useless one; see the comment above -->{' '}<span class="visually-hidden">{hiddenSummary}</span>{/if}</summary>
	{@render children()}
</details>

<style>
	@layer components {
		/*
		 * A closed <details> hides every child but <summary> per the HTML
		 * spec, but that is a user-agent-origin rule, and a plain author
		 * rule on a child -- a layout primitive such as stack-l setting its
		 * own `display` -- wins over user-agent styles regardless of
		 * specificity (CSS cascade origin order). `!important` makes the
		 * closed state explicit rather than depending on a caller's content
		 * never declaring its own display (the bug DataTable's own
		 * disclosure carried before it routed through here, #486).
		 *
		 * `:global()` on the child side, not on `details` itself: the
		 * caller's content arrives through the `children` snippet, so it
		 * carries the caller's own scope class rather than this file's.
		 */
		details:not([open]) > :global(*:not(summary)) {
			display: none !important;
		}

		/*
		 * No marker or appearance override: the platform triangle is what
		 * GOV.UK's own Details component keeps, and this repo has no
		 * established disclosure treatment of its own to depart to.
		 */
		summary {
			cursor: pointer;
			font-family: var(--font-family-base);
			font-size: var(--text-body-sm-size);
			line-height: var(--text-body-sm-leading);
			color: var(--color-on-surface-variant);
			/* WCAG 2.2 target size (minimum), which the axe archetype scan
			   enforces: body-sm alone gives the summary a 21px line box, and
			   --space-6 is 24px exactly, clear of the boundary rather than on
			   it (#459). */
			min-block-size: var(--space-6);
			padding-block: var(--space-1);
		}
	}
</style>

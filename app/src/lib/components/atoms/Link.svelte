<script lang="ts">
	import Icon from './Icon.svelte';
	import type { IconName } from './Icon/manifest.js';

	interface Properties {
		href: string;
		label: string;
		variant?: 'primary' | 'secondary' | 'card' | 'rail' | 'chip' | 'nav' | 'sheet' | 'skip';
		icon?: IconName;
		current?: boolean;
	}

	let { href, label, variant = 'primary', icon, current = false }: Properties = $props();

	// Absolute (http(s)) or protocol-relative hrefs leave the app; every
	// internal route in this codebase is a relative or root-relative path
	// (see `resolve()` call sites), so this alone tells external apart from
	// internal without a redundant `external` prop for callers to keep in sync.
	const isExternal = $derived(/^(https?:)?\/\//.test(href));
</script>

<a
	{href}
	class={variant}
	class:has-icon={Boolean(icon)}
	aria-current={current ? 'page' : undefined}
	target={isExternal ? '_blank' : undefined}
	rel={isExternal ? 'noopener noreferrer' : undefined}
>
	{#if icon}
		<Icon name={icon} size={16} />
	{/if}
	{label}
	{#if isExternal}
		<Icon name="arrow-square-out" size={16} />
		<span class="visually-hidden">(opens in new tab)</span>
	{/if}
</a>

<style>
	@layer components {
		a {
			display: inline-flex;
			align-items: center;
			gap: var(--space-1);
			font-family: var(--font-family-base);
			text-decoration: underline;
		}

		/* A link carrying its own icon (a nav item, a card) reads as a
		   control, not inline prose -- the underline that marks prose links
		   would be visual noise here. */
		a.has-icon {
			text-decoration: none;
		}

		a:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		a[aria-current='page'] {
			color: var(--color-primary);
			font-weight: var(--font-weight-semibold);
		}

		a.primary {
			color: var(--color-primary);
		}

		a.primary:hover {
			color: var(--color-primary-hover);
		}

		a.secondary {
			color: var(--color-on-surface);
		}

		a.secondary:hover {
			color: var(--color-primary);
		}

		/* A block-level tile: icon over label, its own chrome. */
		a.card {
			flex-direction: column;
			align-items: flex-start;
			gap: var(--space-4);
			inline-size: 100%;
			padding: var(--space-5);
			border: var(--border-thin) solid var(--color-outline-variant);
			border-radius: var(--radius);
			background-color: var(--color-surface-bright);
			color: var(--color-on-surface);
			font-size: var(--text-body-size);
			font-weight: var(--font-weight-medium);
			line-height: 1.2;
		}

		a.card:hover {
			border-color: var(--color-primary);
		}

		a.card :global(svg) {
			color: var(--color-on-surface-variant);
		}

		a.card:hover :global(svg) {
			color: var(--color-primary);
		}

		/* An entry in a page's own contents list (RecordDetail's `contents`
		   region, ADR-0018). It is an in-page anchor rather than a route, so
		   it deliberately does not read as a prose link: eight underlined
		   plum links beside a column is the same accent overspend #431 found
		   in the intake step rail and rejected. The left hairline is what
		   collects the entries into a list, so the list frame carries no gap
		   and the rules meet. */
		a.rail {
			display: flex;
			inline-size: 100%;
			padding-block: var(--space-2);
			padding-inline-start: var(--space-3);
			border-inline-start: var(--border-thin) solid var(--color-outline-variant);
			color: var(--color-on-surface-variant);
			font-size: var(--text-body-sm-size);
			font-weight: var(--font-weight-normal);
			text-decoration: none;
		}

		a.rail:hover {
			border-inline-start-color: var(--color-primary);
			color: var(--color-primary);
		}

		/* The same contents list where there is no room beside the column:
		   a wrapping row of targets under the title. A pill rather than a
		   rail entry because it has to survive being read at arm's length in
		   a hospital corridor -- PR-G5, the moment that earned the region. */
		a.chip {
			padding: var(--space-2) var(--space-3);
			border: var(--border-thin) solid var(--color-outline);
			border-radius: var(--radius);
			background-color: var(--color-surface-bright);
			color: var(--color-on-surface);
			font-size: var(--text-label-size);
			font-weight: var(--font-weight-medium);
			text-decoration: none;
		}

		a.chip:hover {
			border-color: var(--color-primary);
			color: var(--color-primary);
		}

		/* A destination in the shell's own nav bar, at either width. It fills
		   the bar's height so the 2px accent rule that marks the current
		   section sits on the bar's own edge rather than floating under the
		   word -- the one place the brief permits a 2px rule (#431). Colour
		   and weight are not the only signal: the route also sets `current`,
		   which puts aria-current="page" on the anchor. */
		a.nav {
			align-self: stretch;
			align-items: center;
			padding-inline: var(--space-4);
			border-block-end: var(--border-active) solid transparent;
			color: var(--color-on-surface-variant);
			font-size: var(--text-body-sm-size);
			text-decoration: none;
			white-space: nowrap;
		}

		a.nav:hover {
			color: var(--color-primary);
		}

		a.nav[aria-current='page'] {
			border-block-end-color: var(--color-primary);
			color: var(--color-primary);
		}

		/* The same destination inside the narrow sheet, where there is a
		   full-width row rather than a bar to sit in, so the rule that marks
		   the current section turns onto the leading edge. */
		a.sheet {
			align-items: center;
			min-block-size: var(--nav-row-height);
			padding-inline: var(--space-5);
			border-inline-start: var(--border-active) solid transparent;
			color: var(--color-on-surface);
			font-size: var(--text-subheading-size);
			text-decoration: none;
		}

		a.sheet:hover {
			background-color: var(--color-surface-container);
		}

		a.sheet[aria-current='page'] {
			border-inline-start-color: var(--color-primary);
			background-color: var(--color-surface-container);
			color: var(--color-primary);
		}

		/* The bypass block (WCAG 2.4.1). It is the first focusable thing in
		   every shell and is invisible until it is focused, which is what
		   lets a six-item nav exist without a keyboard user walking it on
		   every page. Not `display: none` at rest: a hidden element is not
		   focusable, so the link would never be reachable at all. */
		a.skip {
			position: absolute;
			z-index: 1;
			inset-block-start: var(--space-2);
			inset-inline-start: var(--space-2);
			padding: var(--space-2) var(--space-4);
			transform: translateY(calc(-100% - var(--space-4)));
			border: var(--border-thin) solid var(--color-primary);
			border-radius: var(--radius);
			background-color: var(--color-surface-bright);
			color: var(--color-primary);
			font-size: var(--text-body-sm-size);
			font-weight: var(--font-weight-medium);
			text-decoration: none;
		}

		a.skip:focus {
			transform: none;
		}

		/* WCAG-standard clip technique: stays in the accessibility tree and
		   readable by AT/voice-control/translation tools, unlike aria-label
		   which strips real DOM text out of those paths (see Button.svelte). */
		/* tokens:ignore -- the WCAG clip technique's own geometry, not a
		   design value. The 1px box and the -1px pull are what the
		   technique is; a token would imply somebody may retune them. */
		.visually-hidden {
			position: absolute;
			inline-size: 1px;
			block-size: 1px;
			margin: -1px;
			padding: 0;
			overflow: hidden;
			clip: rect(0, 0, 0, 0);
			white-space: nowrap;
			border: 0;
		}
	}
</style>

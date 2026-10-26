<script lang="ts">
	import Icon from './Icon.svelte';
	import type { IconName } from './Icon/manifest.js';

	interface Properties {
		href: string;
		label: string;
		variant?:
			| 'primary'
			| 'secondary'
			| 'card'
			| 'rail'
			| 'chip'
			| 'start'
			| 'nav'
			| 'sheet'
			| 'skip'
			| 'step'
			| 'error'
			| 'brand';
		icon?: IconName;
		current?: boolean;
		/*
		 * Text elsewhere on the page that qualifies this link, joined to it
		 * the way `TextInput` joins a hint to a control. `StepRail` (#464)
		 * needs it: a step's status is a sibling of its link, so without
		 * this a keyboard user tabbing the rail hears "Where Sarah lives"
		 * and never "Not started" -- which is the half that says whether
		 * there is anything to go back for.
		 */
		describedBy?: string;
		/*
		 * Overrides the accessible name `label` would otherwise give the
		 * link, without changing what is drawn. `PortalTopBar`'s own
		 * portal-root link needs it (#310): its visible text stays the
		 * Practice name, so the bar's measured content floor (#564) does
		 * not move, while its accessible name carries the fuller
		 * `engagementLabel` words a screen-reader user needs to tell this
		 * link's destination apart from the "Your care" nav item beside it.
		 */
		ariaLabel?: string;
		/*
		 * Runs as the link is followed, before the navigation. The link
		 * still navigates by its `href`; this is for a screen that must
		 * hand something to the next one outside the URL -- the Client
		 * search gives intake what was typed this way (#1829), since a
		 * Client's details do not belong in an address.
		 */
		onClick?: (event: MouseEvent) => void;
	}

	let {
		href,
		label,
		variant = 'primary',
		icon,
		current = false,
		describedBy,
		ariaLabel,
		onClick
	}: Properties = $props();

	// Absolute (http(s)) or protocol-relative hrefs leave the app; every
	// internal route in this codebase is a relative or root-relative path
	// (see `resolve()` call sites), so this alone tells external apart from
	// internal without a redundant `external` prop for callers to keep in sync.
	const isExternal = $derived(/^(https?:)?\/\//.test(href));
	const text = $derived(isExternal ? `${label} (opens in new tab)` : label);
</script>

<a
	{href}
	class={variant}
	class:has-icon={Boolean(icon)}
	aria-current={current ? 'page' : undefined}
	aria-describedby={describedBy}
	aria-label={ariaLabel}
	target={isExternal ? '_blank' : undefined}
	rel={isExternal ? 'noopener noreferrer' : undefined}
	onclick={onClick}
>
	<!--
		The new-tab words are drawn inside the label, not hidden beside it
		(#1557): GOV.UK's Links style puts "(opens in new tab)" in the link
		text a sighted reader sees too. Inside `.label`, so the words wrap
		with the label at 320px rather than being a flex item that cannot.

		No whitespace between the icons and the label (#1808). In a
		sentence the link is inline, so a space here would be a real space
		in the sentence's accessible text: "(opens in new tab) ." The gap
		is drawn by CSS instead.
	-->
	{#if icon}<Icon name={icon} size={16} />{/if}<span class="label">{text}</span>{#if isExternal}<Icon
			name="arrow-square-out"
			size={16}
		/>{:else if variant === 'start'}<Icon name="arrow-right" size={20} />{/if}
</a>

<style>
	@layer components {
		/* The intrinsic-width rule `Button.svelte` explains in full, for the
		   same reason: a link's hit area running the width of the page is an
		   affordance pointing at nothing. */
		:global(stack-l) > a {
			align-self: start;
		}

		/*
		 * A link on its own -- a screen's one action, a Back link, an entry
		 * in a list -- is a target, and the brief's Density rule holds it
		 * to 44px in both directions whatever its words measure (#1828).
		 * `min-*-size` rather than padding, so the box grows to fit long
		 * words but never shrinks below the floor; `align-items` keeps the
		 * text centered in the taller box.
		 *
		 * The inline floor is on `.label` below, not here (#1851). A flex
		 * item's own minimum is its min-content -- its words -- and an
		 * explicit `min-inline-size` on the link replaces it rather than
		 * adding to it, so a link in a row with too little room shrank to
		 * 44px and its words drew over the next item's. What the link needs
		 * is the larger of the two, and `max(44px, min-content)` is not CSS:
		 * a math function takes no sizing keyword, and `calc-size()`, which
		 * would, is Chromium's alone. A floor on the label is part of the
		 * link's min-content instead, so the link's own minimum is its words
		 * or 44px, whichever is wider, in every engine.
		 */
		a {
			display: inline-flex;
			align-items: center;
			gap: var(--space-1);
			min-block-size: var(--hit-target-min);
			/* Never wider than its container: `.label` says why. */
			max-inline-size: 100%;
			font-family: var(--font-family-base);
			text-decoration: underline;
		}

		/*
		 * A link inside a paragraph is part of a sentence, and the brief
		 * excepts it from the 44px floor the way WCAG 2.2's 2.5.8 does
		 * ("Inline"): a 44px box would push the sentence's lines apart.
		 * So here it is plain inline text, not a flex box (#1808): an
		 * inline-flex box is an item of its own in the accessible text, and
		 * a full stop after it was read as " .". `display: inline` also
		 * turns off `min-*-size`, which an inline box does not take.
		 *
		 * The rule is the paragraph, not the words beside the link: a link
		 * that stands alone is not written inside a `<p>`, which
		 * `standaloneLink.usage.spec.ts` holds.
		 */
		:global(p) a {
			display: inline;
		}

		/* The icon is inline too, for the same reason: `Icon` draws it as
		   an inline-block, and the accessible-text walk puts a space beside
		   an inline-block even when it is hidden. `gap` has nothing to act
		   on in an inline box, so the space between a label and its icon is
		   a margin there instead. */
		:global(p) a > :global(svg) {
			display: inline;
			vertical-align: middle;
		}

		:global(p) a > :global(svg:first-child) {
			margin-inline-end: var(--space-1);
		}

		:global(p) a > :global(svg:last-child) {
			margin-inline-start: var(--space-1);
		}

		/* A link in a table row takes the row's height as its floor, in
		   both directions (#1843, the brief's Density section):
		   `--row-target-min` is the row less its 1px rule, 39px when every
		   input is fine and 44px when any input can tap, so the row and
		   its targets are always the same height. The inline floor
		   subtracts the variant's own padding, as the 44px rule on
		   `.label` below does. */
		:global(td) a {
			min-block-size: var(--row-target-min);
		}

		:global(td) .label {
			min-inline-size: calc(
				var(--row-target-min) - var(--link-inset-start, 0rem) - var(--link-inset-end, 0rem)
			);
		}

		/*
		 * The 44px inline floor (#1828), held here rather than on the link
		 * for the reason the `a` rule gives (#1851). `--link-inset-start`
		 * and `--link-inset-end` are a variant's own inline padding, which
		 * its `padding-inline` reads too, so the label's floor is what is
		 * left of 44px once that padding is drawn: a short label
		 * takes the 44px box it did, and a long one takes no room it did
		 * not. A label shorter than its floor leaves the rest of its box
		 * empty after the words, which draws as nothing.
		 *
		 * A flex item's automatic minimum size is its min-content size
		 * (the CSS spec's own default), and a URL used as a label has no
		 * character a browser will break a line on -- so the label refused
		 * to shrink below the URL's full width and pushed the anchor past
		 * its frame (#548). An explicit `min-inline-size` overrides that
		 * floor -- the same override `TextInput`, `MembershipFields`,
		 * `QuestionPage` and `FormPage` use on their own shrinking flex
		 * item -- so the label shrinks to its floor and no further.
		 *
		 * `break-word`, not `anywhere` (#1851). Both break a URL, which has
		 * no space or hyphen to wrap at, inside the width the label is
		 * given. `anywhere` also counts every character as a break when the
		 * label's min-content is measured, so a link in a row with too
		 * little room broke its words mid-letter before the row ever
		 * overflowed. `break-word` leaves the min-content at the longest
		 * word, so the link keeps its words and the row wraps or
		 * overflows. The link's own `max-inline-size: 100%` is what stops
		 * that longest "word" -- a URL -- from pushing it past its frame.
		 * The anchor stays a `nowrap` flex row (no `flex-wrap` here), so
		 * the label is the only thing that wraps -- the leading icon and the
		 * trailing external-link icon stay pinned beside it rather than ever
		 * landing on a row of their own. The "(opens in new tab)" words are
		 * part of the label (#1557), so they wrap with it.
		 */
		.label {
			min-inline-size: calc(
				var(--hit-target-min) - var(--link-inset-start, 0rem) - var(--link-inset-end, 0rem)
			);
			overflow-wrap: break-word;
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

		/* An entry in the error summary (#467): the one reason a submit was
		   refused, and the way to the control that caused it. Red and bold
		   because it has to read as part of the error treatment rather than
		   as ordinary prose inside a red box, which is GOV.UK's own call.
		   It is a variant rather than a `class` an ErrorSummary passes down
		   because ADR-0018 allows no such passthrough -- naming the
		   treatment here is what keeps it the same in every consumer, the
		   same reason `step` and `chip` exist. */
		a.error {
			color: var(--color-error);
			font-weight: var(--font-weight-semibold);
		}

		a.error:focus-visible {
			outline-color: var(--color-error);
		}

		a.secondary {
			color: var(--color-on-surface);
		}

		a.secondary:hover {
			color: var(--color-primary);
		}

		/* A shell's own name, doubling as a link back to its root
		   (`PortalTopBar`, #310) -- plain text at rest, so it reads as the
		   identity it always was rather than as ordinary prose, with a
		   hover/focus treatment because it is a control now. */
		a.brand {
			color: var(--color-on-surface);
			font-size: var(--text-subheading-size);
			font-weight: var(--font-weight-semibold);
			text-decoration: none;
		}

		a.brand:hover {
			color: var(--color-primary);
		}

		/* A block-level tile: icon over label, its own chrome. */
		a.card {
			flex-direction: column;
			align-items: flex-start;
			gap: var(--space-4);
			inline-size: 100%;
			--link-inset-start: var(--space-5);
			--link-inset-end: var(--space-5);
			padding-block: var(--space-5);
			padding-inline: var(--link-inset-start) var(--link-inset-end);
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
			--link-inset-start: var(--space-3);
			padding-inline-start: var(--link-inset-start);
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
			--link-inset-start: var(--space-3);
			--link-inset-end: var(--space-3);
			padding-block: var(--space-2);
			padding-inline: var(--link-inset-start) var(--link-inset-end);
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

		/* The applied filter (#2019). `a.chip` sets its own color and
		   weight, so the bare `a[aria-current]` rule above never reached a
		   chip, and the applied one looked like the others. Its border is
		   drawn twice as thick with an inset shadow rather than a wider
		   border, so the chip does not grow and the row does not shift as
		   the filter changes; the weight is the signal that is not color. */
		a.chip[aria-current='page'] {
			border-color: var(--color-primary);
			box-shadow: inset 0 0 0 var(--border-thin) var(--color-primary);
			background-color: var(--color-surface-container);
			color: var(--color-primary);
			font-weight: var(--font-weight-semibold);
		}

		/*
		 * GOV.UK's start button (#2019): a link drawn as the primary
		 * `Button`, for the one navigation that starts a screen's task --
		 * "Invite a Staff member", "Add your first Client". It is a link
		 * because it goes somewhere rather than doing something, and the
		 * arrow after the label is GOV.UK's own mark for it. Used once on a
		 * screen at most, and never beside a primary `Button`: the accent
		 * is spent once (the brief's Von Restorff row). Its box and type
		 * match `Button`'s `primary` at `md`.
		 */
		a.start {
			justify-content: center;
			gap: var(--space-2);
			--link-inset-start: var(--space-4);
			--link-inset-end: var(--space-4);
			padding-block: var(--space-2);
			padding-inline: var(--link-inset-start) var(--link-inset-end);
			border: var(--border-thin) solid transparent;
			border-radius: var(--radius);
			background-color: var(--color-primary);
			color: var(--color-on-primary);
			font-size: var(--text-body-size);
			font-weight: var(--font-weight-medium);
			text-decoration: none;
		}

		a.start:hover {
			background-color: var(--color-primary-hover);
		}

		/* A destination in the shell's own nav bar, at either width. It fills
		   the bar's height so the 2px accent rule that marks the current
		   section sits on the bar's own edge rather than floating under the
		   word -- the one place the brief permits a 2px rule (#431). Color
		   and weight are not the only signal: the route also sets `current`,
		   which puts aria-current="page" on the anchor. */
		a.nav {
			align-self: stretch;
			align-items: center;
			--link-inset-start: var(--space-4);
			--link-inset-end: var(--space-4);
			max-inline-size: none;
			padding-inline: var(--link-inset-start) var(--link-inset-end);
			border-block-end: var(--border-active) solid transparent;
			color: var(--color-on-surface-variant);
			font-size: var(--text-body-sm-size);
			text-decoration: none;
			/* Its words never wrap, so nothing caps it below them: a row
			   with too little room for its sections overflows instead. */
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
			--link-inset-start: var(--space-5);
			--link-inset-end: var(--space-5);
			padding-inline: var(--link-inset-start) var(--link-inset-end);
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

		/* A step in a journey's own rail (`StepRail`, #464). Unlike `rail`,
		   whose entries are in-page anchors, a step is a route -- so it
		   takes the shell's nav treatment rather than the link treatment:
		   neutral text with the 2px bar the brief reserves for an active
		   nav marker. #432's first pass used ordinary prose links, which
		   put five accent-colored titles and ten accent sub-links on one
		   page, against the brief's Von Restorff rule. The accent is now
		   spent once in the rail -- on the step you are on -- and once on
		   `Continue`. */
		a.step {
			align-items: baseline;
			inline-size: 100%;
			padding-block: var(--space-2);
			--link-inset-start: var(--space-4);
			padding-inline-start: var(--link-inset-start);
			border-inline-start: var(--border-active) solid transparent;
			color: var(--color-on-surface);
			font-size: var(--text-body-sm-size);
			font-weight: var(--font-weight-medium);
			text-decoration: none;
		}

		a.step:hover {
			background-color: var(--color-surface-container);
		}

		a.step[aria-current='page'] {
			border-inline-start-color: var(--color-primary);
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
			--link-inset-start: var(--space-4);
			--link-inset-end: var(--space-4);
			padding-block: var(--space-2);
			padding-inline: var(--link-inset-start) var(--link-inset-end);
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
	}
</style>

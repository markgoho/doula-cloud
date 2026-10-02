<script lang="ts">
	/*
	 * The signed-out landing at `/` (#1645): two panels, a welcome and the
	 * doors. ADR-0018's 2026-10-02 amendment records why this is a Template
	 * of its own rather than `EntryPage`, and why one consumer clears the
	 * bar here.
	 *
	 * The welcome panel holds the mark, a greeting and one line under it.
	 * The greeting is a paragraph, never a heading: the `<h1>` names what
	 * the page is for (ADR-0021), and it is the doors panel's. The greeting
	 * is set at the brief's `display` step, which until this Template was
	 * spent only by `OverviewHub`'s title; it lives here for the same reason
	 * it lives there -- a route cannot reach it.
	 *
	 * The Template takes the greeting as a string and does not read a clock.
	 * The route reads the clock once, before the first paint, so the page
	 * never paints one greeting and swaps to another.
	 *
	 * The two panels are a wrapping grid (`grid-l`), so they sit side by
	 * side where each can have `--landing-panel-min` and stack where they
	 * cannot -- read off the space the page has, never the window (ADR-0024).
	 * The edge between them is the grid's own one-pixel gap over an
	 * `outline-variant` ground, so it is a vertical rule beside and a
	 * horizontal rule above, with no query to pick which.
	 *
	 * The panels fill the window below the bar (#1653). The shell owns that
	 * height and hands it down; this Template only asks for it.
	 */
	import type { Snippet } from 'svelte';
	import CloudMark from '#lib/components/atoms/CloudMark.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';

	interface Properties {
		title: string;
		/**
		 * The time-of-day line, already chosen by the caller.
		 */
		greeting: string;
		/**
		 * The one line under the greeting.
		 */
		lede: string;
		/**
		 * Everything under the title in the doors panel.
		 */
		content: Snippet;
	}

	let { title, greeting, lede, content }: Properties = $props();
</script>

<PageTitle page={title} />

<container-l data-fills-main>
	<grid-l min="var(--landing-panel-min)" space="var(--border-thin)">
		<div class="panel welcome">
			<stack-l space="var(--space-8)">
				<div class="mark"><CloudMark size="xl" /></div>
				<stack-l space="var(--space-3)">
					<p class="greeting">{greeting}</p>
					<p class="lede">{lede}</p>
				</stack-l>
			</stack-l>
		</div>
		<div class="panel doors">
			<stack-l space="var(--space-7)">
				<Heading level={1} variant="page" text={title} />
				{@render content()}
			</stack-l>
		</div>
	</grid-l>
</container-l>

<style>
	@layer components {
		/* The shell gives this Template the window's remaining height
		   (`data-fills-main`, base.css, #1653). A grid here passes that
		   height to the panels, whose own centering then has room to work. */
		container-l {
			display: grid;
		}

		grid-l {
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-outline-variant);
		}

		.panel {
			display: flex;
			flex-direction: column;
			justify-content: center;
			padding: var(--space-12) var(--page-gutter);
		}

		/* The welcome is one block, centered side to side in its panel
		   (#1653): the mark and the two lines keep a shared left edge, and
		   the block as a whole sits in the middle. On a wide window the
		   panel is far wider than a greeting, and a block held to the left
		   edge left most of it empty. It is centered at every width,
		   stacked or side by side: one rule, so the welcome reads the same
		   on a phone as on a wide monitor. The doors stay where they are:
		   they are a column of controls with a width of their own. */
		.welcome {
			align-items: center;
			background-color: var(--color-surface-bright);
		}

		.doors {
			background-color: var(--color-surface);
		}

		.doors > stack-l {
			max-inline-size: var(--form-max);
		}

		/* About 200px, and never more than about 55% of the panel: the mark
		   scales inside this box rather than overflowing it. The cap is
		   `cqi`, measured against the page (`container-l`), and not a
		   percentage: the welcome block shrinks to its content, so a
		   percentage of it would size the mark off the greeting's length.
		   Stacked, the panel is the page, so 55cqi is 55% of the panel.
		   Side by side, each panel is at least `--landing-panel-min`, where
		   the mark's own 200px is already under half. */
		.mark {
			max-inline-size: 55cqi;
		}

		p {
			margin: 0;
			font-family: var(--font-family-base);
			overflow-wrap: anywhere;
		}

		.greeting {
			color: var(--color-on-surface);
			font-size: var(--text-display-size);
			font-weight: var(--text-display-weight);
			line-height: var(--text-display-leading);
			letter-spacing: var(--text-display-tracking);
		}

		/* The heading step in regular weight: secondary to the greeting by
		   tone and weight, not by a step of its own. */
		.lede {
			color: var(--color-on-surface-variant);
			font-size: var(--text-heading-size);
			font-weight: var(--font-weight-normal);
			line-height: var(--text-heading-leading);
			letter-spacing: var(--text-heading-tracking);
		}
	}
</style>

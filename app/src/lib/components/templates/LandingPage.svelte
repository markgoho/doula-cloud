<script lang="ts">
	/*
	 * The signed-out landing at `/` (#1645). ADR-0018's 2026-10-02 amendment
	 * records why this is a Template of its own rather than `EntryPage`.
	 *
	 * One column at every width: the `<h1>` names what the page is for
	 * (ADR-0021), the greeting and the lede are the one line under it, and
	 * the content (the doors) follows. Visual order is reading order, and on
	 * a phone the doors sit near the top rather than under a welcome panel.
	 *
	 * The Template takes the greeting as a string and does not read a clock.
	 * The route reads the clock once, before the first paint, so the page
	 * never paints one greeting and swaps to another.
	 *
	 * The column fills the window below the bar (#1653) and centers in it.
	 */
	import type { Snippet } from 'svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';

	interface Properties {
		title: string;
		/**
		 * The time-of-day line, already chosen by the caller.
		 */
		greeting: string;
		/**
		 * Follows the greeting on the line under the title.
		 */
		lede: string;
		/**
		 * Everything under the title and its line.
		 */
		content: Snippet;
	}

	let { title, greeting, lede, content }: Properties = $props();
</script>

<PageTitle page={title} />

<container-l data-fills-main>
	<div class="column">
		<stack-l space="var(--space-8)">
			<stack-l space="var(--space-3)">
				<Heading level={1} variant="page" text={title} />
				<p class="lede"><span class="greeting">{greeting}</span> {lede}</p>
			</stack-l>
			{@render content()}
		</stack-l>
	</div>
</container-l>

<style>
	@layer components {
		/* The shell gives this Template the window's remaining height
		   (`data-fills-main`, base.css, #1653). */
		container-l {
			display: grid;
			background-color: var(--color-surface);
		}

		.column {
			display: flex;
			flex-direction: column;
			justify-content: center;
			inline-size: 100%;
			max-inline-size: var(--form-max);
			margin-inline: auto;
			padding: var(--space-8) var(--page-gutter);
		}

		.lede {
			margin: 0;
			color: var(--color-on-surface-variant);
			font-family: var(--font-family-base);
			font-size: var(--text-subheading-size);
			font-weight: var(--font-weight-normal);
			line-height: var(--text-subheading-leading);
			letter-spacing: var(--text-subheading-tracking);
			overflow-wrap: anywhere;
		}

		.greeting {
			color: var(--color-on-surface);
		}
	}
</style>

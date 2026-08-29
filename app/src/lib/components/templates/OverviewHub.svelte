<script lang="ts">
	/*
	 * Archetype B -- the overview hub. ADR-0018.
	 *
	 * `isEmpty` and `empty` are required, not optional, and that is the whole
	 * reason this layer exists. `docs/journeys/evaluator-doula.md` names the
	 * Practice landing page as Tasha Bell's abandon point -- "an empty filing
	 * cabinet, not proof" -- because nobody had to think about the zero-data
	 * case. A Template that cannot be instantiated without an empty state
	 * makes forgetting it a type error.
	 *
	 * The title is the one place in the app the brief's `display` type step is
	 * allowed, which is why `Heading` deliberately cannot reach it (#417): a
	 * one-per-app rule that depends on everyone remembering it is not a rule,
	 * so the step lives here and nowhere a route can pass it.
	 */
	import type { Snippet } from 'svelte';

	interface Properties {
		title: string;
		primary: Snippet;
		secondary?: Snippet;
		isEmpty: boolean;
		empty: Snippet;
	}

	let { title, primary, secondary, isEmpty, empty }: Properties = $props();
</script>

<container-l>
	<center-l max="var(--page-max)" gutters="var(--page-gutter)">
		<stack-l space="var(--space-8)">
			<h1>{title}</h1>

			{#if isEmpty}
				<div class="empty">{@render empty()}</div>
			{:else}
				<div class="body" class:has-secondary={secondary}>
					<stack-l space="var(--space-6)">{@render primary()}</stack-l>
					{#if secondary}
						<aside><stack-l space="var(--space-6)">{@render secondary()}</stack-l></aside>
					{/if}
				</div>
			{/if}
		</stack-l>
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		h1 {
			margin: 0;
			font-family: var(--font-family-base);
			font-size: var(--text-display-size);
			font-weight: var(--text-display-weight);
			line-height: var(--text-display-leading);
			letter-spacing: var(--text-display-tracking);
			color: var(--color-on-surface);
		}

		.body {
			display: grid;
			gap: var(--space-8);
		}

		/*
		 * Container query, not a media query: ADR-0003 makes container
		 * queries the default and the rail depends on how wide the page
		 * frame is, not how wide the window is. The threshold is the
		 * narrowest width at which a 20rem rail still leaves the primary
		 * column above its own comfortable measure.
		 */
		@container (min-width: 60rem) {
			.body.has-secondary {
				grid-template-columns: minmax(0, 1fr) 20rem;
			}
		}
	}
</style>

<script lang="ts">
	/*
	 * Archetype D -- the multi-section record detail. ADR-0018.
	 *
	 * `sections` is an array rather than a run of named Snippet props because
	 * the page that forced this Template has a *variable* number of them: the
	 * staff Engagement detail is Visits, then N Plan sections, then Contract,
	 * Invoices, Offers and Messages. Named regions cannot express N. It is
	 * `DataTable.rowActions`'s shape generalised -- the third confirmed use of
	 * Snippets as an API, after `LabeledField` and `DataTable`.
	 */
	import type { Snippet } from 'svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';

	interface Section {
		heading: string;
		content: Snippet;
	}

	interface Properties {
		title: string;
		summary?: Snippet;
		actions?: Snippet;
		sections: Section[];
	}

	let { title, summary, actions, sections }: Properties = $props();
</script>

<container-l>
	<center-l max="var(--page-max)" gutters="var(--page-gutter)">
		<stack-l space="var(--space-8)">
			<!--
				A plain div, not <header>: a <header> outside article/aside/main/
				nav/section maps to the `banner` landmark, and the banner is the
				shell's (#431), not a page's. A Template renders no chrome.
			-->
			<div class="record-header">
				<cluster-l space="var(--space-4)" justify="space-between" align="baseline">
					<Heading level={1} variant="page" text={title} />
					{#if actions}
						<cluster-l space="var(--space-2)" align="center">{@render actions()}</cluster-l>
					{/if}
				</cluster-l>
				{#if summary}
					<div class="summary">{@render summary()}</div>
				{/if}
			</div>

			{#each sections as section (section.heading)}
				<section>
					<stack-l space="var(--space-4)">
						<Heading level={2} variant="section" text={section.heading} />
						{@render section.content()}
					</stack-l>
				</section>
			{/each}
		</stack-l>
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		/*
		 * Law of Proximity: the summary belongs to the title, so it sits
		 * nearer to it than the first section does -- --space-4 here against
		 * the --space-8 the stack puts between sections. That difference is
		 * what groups the header, not a border.
		 */
		.summary {
			margin-block-start: var(--space-4);
		}
	}
</style>

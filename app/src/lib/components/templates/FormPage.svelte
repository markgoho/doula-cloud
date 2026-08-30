<script lang="ts">
	/*
	 * Archetype E -- the long form. ADR-0018.
	 *
	 * `fieldsets` is ADR-0017's shape: the twelve-column structural core is
	 * one fieldset, and each Practice-defined section is another appended
	 * below it -- the pattern the #406 survey found in Cliniko and endorsed as
	 * the one matching ADR-0017. So the count is not knowable in advance and
	 * the region is an array.
	 *
	 * This Template renders no <form> element and owns no submit handler. The
	 * route wraps the Template in its own <form>, which is valid HTML and is
	 * how GOV.UK builds the same page. Submission is behaviour, and a Template
	 * owns page-level arrangement and nothing else.
	 *
	 * `errorSummary` renders **above** the title, which is GOV.UK's own
	 * position for it and the one `QuestionPage` already takes. It sat
	 * under the title until #467: the reader has to meet the problem
	 * before the page's own name, because on a refused submit the problem
	 * is why she is back here at all. This Template owns *where* it goes
	 * and none of what goes in it -- ADR-0018's rule that a fixed region is
	 * a named Snippet prop, so `ErrorSummary` lives in exactly one place
	 * (#467) rather than being rebuilt by each Template that positions it.
	 */
	import type { Snippet } from 'svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';

	/*
	 * `legend` is optional because a form can have a group that names
	 * nothing: `invite` (#425) asks for one email address and then a
	 * Membership, and neither group has a Practice-given name to print.
	 * A <fieldset> whose <legend> is empty announces a group with no name,
	 * which is worse than no group at all, so an unnamed entry renders as
	 * a plain stack and no <fieldset> element is emitted.
	 */
	interface Fieldset {
		legend?: string;
		content: Snippet;
	}

	interface Properties {
		title: string;
		intro?: Snippet;
		fieldsets: Fieldset[];
		/**
		 * GOV.UK's error summary, positioned by this Template and built by
		 * the route (#467). Nothing renders here when it is absent -- not an
		 * empty box, not a hidden live region.
		 */
		errorSummary?: Snippet;
		actions: Snippet;
	}

	let { title, intro, fieldsets, errorSummary, actions }: Properties = $props();
</script>

<container-l>
	<center-l max="var(--form-max)" gutters="var(--page-gutter)">
		<stack-l space="var(--space-7)">
			{#if errorSummary}
				{@render errorSummary()}
			{/if}

			<Heading level={1} variant="page" text={title} />

			{#if intro}
				<div class="intro">{@render intro()}</div>
			{/if}

			<!--
				Keyed on index, not on the legend: two groups may share a
				legend, and two unnamed groups share `undefined`, so a
				legend key collides. The array is positional anyway -- a
				fieldset has no identity beyond where it sits.
			-->
			{#each fieldsets as fieldset, index (index)}
				{#if fieldset.legend === undefined}
					<stack-l space="var(--space-5)">{@render fieldset.content()}</stack-l>
				{:else}
					<fieldset>
						<legend>{fieldset.legend}</legend>
						<stack-l space="var(--space-5)">{@render fieldset.content()}</stack-l>
					</fieldset>
				{/if}
			{/each}

			<cluster-l space="var(--space-3)" align="center">{@render actions()}</cluster-l>
		</stack-l>
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		/*
		 * The brief's "airy forms" half: consecutive fields 20px apart
		 * (--space-5, on the stack inside each fieldset) and a labelled group
		 * 28px from the next (--space-7, on the outer stack). A fieldset
		 * carries no border of its own -- the Law of Proximity does the
		 * grouping, and the direction drops boxes that only exist to look
		 * busy.
		 */
		fieldset {
			margin: 0;
			padding: 0;
			border: 0;
			min-inline-size: 0;
		}

		legend {
			padding: 0;
			margin-block-end: var(--space-4);
			font-family: var(--font-family-base);
			font-size: var(--text-heading-size);
			font-weight: var(--text-heading-weight);
			line-height: var(--text-heading-leading);
			letter-spacing: var(--text-heading-tracking);
			color: var(--color-on-surface);
		}
	}
</style>

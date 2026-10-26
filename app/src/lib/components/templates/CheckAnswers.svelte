<script module lang="ts">
	export interface Answer {
		label: string;
		value: string;
		changeHref: string;
		/**
		 * What the Change link changes, in words -- "given name", "email
		 * address". Every row's visible text is the same word, so without
		 * this a screen reader user listing the page's links hears "Change"
		 * nineteen times; GOV.UK's own answer is visually-hidden text after
		 * it, which is what this becomes.
		 */
		changes: string;
	}

	export interface AnswerSection {
		/**
		 * Unset for a group that needs no heading (#2033): GOV.UK's Check
		 * answers heads only answers that form a group, so the built-in
		 * questions, one row each, sit under the page's own title.
		 */
		heading?: string;
		answers: Answer[];
	}
</script>

<script lang="ts">
	/*
	 * Archetype E, second half -- the summary page that ends a question
	 * sequence. ADR-0018, amended on #464.
	 *
	 * ## The rows
	 *
	 * A check-answers row is a label, a value and an action. #464 kept the
	 * row markup here until a second page wanted it; the Engagement page
	 * did (#1998), so the rows are `molecules/SummaryList.svelte`, GOV.UK's
	 * Summary list, which every key/value list renders through (#2021).
	 * Its key column is a share of the list, so each section's values
	 * start on the same line down the page.
	 *
	 * ## One width
	 *
	 * The column is the question pages' column, --form-max, centered, with
	 * the journey rail inside it (#2005). There was an `isWide` that
	 * dropped the cap for a long answer list, as GOV.UK allows; it is gone
	 * because `SummaryList` caps its rows at --measure (#2021), narrower
	 * than --form-max, so a wider column widened nothing but the space
	 * beside the answers.
	 */
	import type { Snippet } from 'svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import BackLink from '#lib/components/molecules/BackLink.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import StepRail, { type JourneyStep } from '#lib/components/organisms/StepRail.svelte';

	interface Properties {
		journey: string;
		steps: JourneyStep[];
		backHref: string;
		title: string;
		/**
		 * The Staff side leaves this unset and gets the product name; the
		 * Client portal passes its Practice's name (#431, #487).
		 */
		serviceName?: string;
		caption?: string;
		/**
		 * Positioned here, built by the route (#467). Same rule as
		 * `QuestionPage`.
		 */
		errorSummary?: Snippet;
		sections: AnswerSection[];
		/**
		 * GOV.UK's declaration: what pressing the button agrees to, read
		 * after the answers and directly before the actions (#2049). The
		 * route writes the words and joins them to its button as the
		 * button's description; this Template only places them.
		 */
		declaration?: Snippet;
		actions: Snippet;
	}

	let {
		journey,
		steps,
		backHref,
		title,
		serviceName,
		caption,
		errorSummary,
		sections,
		declaration,
		actions
	}: Properties = $props();
</script>

<PageTitle page={title} {serviceName} isError={Boolean(errorSummary)} />

<container-l>
	<!-- No cap on the frame: the column carries its own cap (#541). -->
	<center-l max="none" gutters="var(--page-gutter)">
		<div class="column">
				<stack-l space="var(--space-6)">
					<BackLink href={backHref} />

					<!--
						In the column, under Back and over the title: the same
						place and the same shape as on every question page of
						the journey (#2005), so the step indicator does not
						move or change form on the last page. It used to sit
						in a `sidebar-l` column of its own beside the answers,
						open. It starts closed here as everywhere: the answers
						under it are already the whole journey at a glance
						(#432). Opened, every completed step lists its
						questions.
					-->
					<StepRail {journey} {steps} expand="completed" />

					{#if errorSummary}
						{@render errorSummary()}
					{/if}

					<div>
						{#if caption}
							<span class="caption">{caption}</span>
						{/if}
						<Heading level={1} variant="page" text={title} />
					</div>

					<stack-l space="var(--space-8)">
						<!--
							Keyed on index, not on the heading: a Practice names its
							own sections and two may share a name, which is the same
							defect #464 fixed in the key/value list.
						-->
						{#each sections as section, sectionIndex (sectionIndex)}
							<section>
								<stack-l space="var(--space-4)">
									{#if section.heading}
										<Heading level={2} variant="section" text={section.heading} />
									{/if}
									<!--
										The rows are `SummaryList`'s (#1998). The Change is a
										link back to the question, described by the
										visually-hidden words SummaryList draws, so the
										announced name is "Change, given name".

										`secondary`, not the plum prose link GOV.UK uses: a
										summary page carries one Change link per row, and the
										brief's Von Restorff rule spends the accent once per
										screen -- which here is `Save this client`. Nineteen
										plum links is the same overspend #432 rejected in the
										step rail's first pass. Neutral, underlined, and plum
										on hover keeps the affordance without the noise.
									-->
									<SummaryList rows={section.answers}>
										{#snippet action(answer, describedBy)}
											<Link href={answer.changeHref} label="Change" variant="secondary" {describedBy} />
										{/snippet}
									</SummaryList>
								</stack-l>
							</section>
						{/each}
					</stack-l>

					{#if declaration}
						<stack-l space="var(--space-4)">{@render declaration()}</stack-l>
					{/if}

					<cluster-l space="var(--space-4)" align="center">{@render actions()}</cluster-l>
				</stack-l>
		</div>
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		/*
		 * Centered and capped at --form-max, as `QuestionPage`'s column
		 * is: a summary of answers has nothing more to put in a wider
		 * window (#543).
		 */
		.column {
			max-inline-size: var(--form-max);
			margin-inline: auto;
		}

		.caption {
			display: block;
			margin-block-end: var(--space-1);
			font-size: var(--text-body-size);
			color: var(--color-on-surface-muted);
		}
	}
</style>

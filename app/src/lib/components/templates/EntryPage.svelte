<script lang="ts">
	/*
	 * Archetype A -- the unauthenticated entry screen. ADR-0018's Scope
	 * section named this archetype and deliberately left it: "A
	 * (unauthenticated entry) ... deliberately left". #490 gives it a
	 * Template.
	 *
	 * Eight routes share this shape: login and accept-invite on both the
	 * Staff and portal sides, signup, forgot-password and reset-password,
	 * and the pre-account Offer read (#1222, joining the last three).
	 * Each already has a product or Practice name on the bar above
	 * (`SignedOutTopBar`, #452's shell), so this Template's own job is
	 * the title, the error summary's position, and the frame around
	 * whatever the route asks for next -- one form, two forms that
	 * switch, a Notice, a "choose a Practice" or "choose an Engagement"
	 * picker, a read-only record summary with decision controls. That
	 * variety is exactly the region-internal arrangement ADR-0018 leaves
	 * to the page; this Template owns only the page-level part.
	 *
	 * Top-aligned in a --form-max column, not centered in the viewport.
	 * Every other Template in this layer -- FormPage, QuestionPage,
	 * CheckAnswers, RecordDetail, OverviewHub, ErrorPage -- renders in
	 * normal flow under `padding-block: var(--space-8)`, and none of them
	 * owns the viewport's own height. Centring this one vertically would
	 * be a second layout mechanism for a single archetype, sized against
	 * a bar (`SignedOutTopBar`) whose height this Template has no
	 * business knowing -- the kind of anonymous escape hatch ADR-0018's
	 * "two named exits" section exists to prevent. `--form-max` is the
	 * same token `FormPage` already spends on a form column, so no new
	 * width enters the app for this.
	 */
	import type { Snippet } from 'svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';

	interface Properties {
		title: string;
		/**
		 * GOV.UK's error summary, positioned by this Template and built by
		 * the route (#467). Nothing renders here when it is absent.
		 */
		errorSummary?: Snippet;
		/**
		 * Everything under the title: the credentials form, a second step,
		 * a Notice, a picker. A single region because the eight routes this
		 * Template serves differ in what that is -- accept-invite switches
		 * between two forms and a read-only summary, the offer read
		 * switches between an access-code form and a read-only record with
		 * decision controls, the plain logins never do -- and a Template
		 * does not get to know which.
		 *
		 * The region takes groups, not loose items (#2020): each child is
		 * spaced from the next at `--space-group`, the gap between the
		 * title and the form. Things that belong together -- two links
		 * offered side by side, the paragraphs of one explanation -- go in
		 * one `stack-l` (or, for links that can share a line, one
		 * `cluster-l`) at `space="var(--space-related)"`, so they read as
		 * one group rather than as groups of their own.
		 */
		content: Snippet;
		/**
		 * One line under the title that says where the person is (#2025):
		 * which door this is, which address is signing in. A person who
		 * opens a link from an email on a phone arrives cold, and a title
		 * and a Continue button alone do not say what Continue does. A
		 * string, not a region: the brief's Voice rule 5 allows a title one
		 * short line, and a string cannot grow into a paragraph.
		 */
		lede?: string;
	}

	let { title, errorSummary, content, lede }: Properties = $props();
</script>

<PageTitle page={title} isError={Boolean(errorSummary)} />

<container-l>
	<center-l max="var(--form-max)" gutters="var(--page-gutter)">
		<stack-l space="var(--space-group)">
			{#if errorSummary}
				{@render errorSummary()}
			{/if}

			{#if lede}
				<!--
					The lede belongs to the title, so the two are one group at
					`--space-related` (#2020's rule), not two at `--space-group`.
					It can hold an address, which wraps at 320px rather than
					pushing the page wide.
				-->
				<stack-l space="var(--space-related)">
					<Heading level={1} variant="page" text={title} />
					<div class="lede"><Text text={lede} /></div>
				</stack-l>
			{:else}
				<Heading level={1} variant="page" text={title} />
			{/if}

			{@render content()}
		</stack-l>
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		.lede {
			overflow-wrap: anywhere;
		}
	}
</style>

<script lang="ts">
	/*
	 * Archetype A -- the unauthenticated entry screen. ADR-0018's Scope
	 * section named this archetype and deliberately left it: "A
	 * (unauthenticated entry) ... deliberately left". #490 gives it a
	 * Template.
	 *
	 * Five routes share this shape: login and accept-invite on both the
	 * Staff and portal sides, and signup. Each already has a product or
	 * Practice name on the bar above (`SignedOutTopBar`, #452's shell),
	 * so this Template's own job is the title, the error summary's
	 * position, and the frame around whatever the route asks for next --
	 * one form, two forms that switch, a Notice, a "choose a Practice" or
	 * "choose an Engagement" picker. That variety is exactly the
	 * region-internal arrangement ADR-0018 leaves to the page; this
	 * Template owns only the page-level part.
	 *
	 * Top-aligned in a --form-max column, not centred in the viewport.
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
		 * a Notice, a picker. A single region because the five routes this
		 * Template serves differ in what that is -- accept-invite switches
		 * between two forms and a read-only summary, the plain logins never
		 * do -- and a Template does not get to know which.
		 */
		content: Snippet;
	}

	let { title, errorSummary, content }: Properties = $props();
</script>

<PageTitle page={title} isError={Boolean(errorSummary)} />

<container-l>
	<center-l max="var(--form-max)" gutters="var(--page-gutter)">
		<stack-l space="var(--space-7)">
			{#if errorSummary}
				{@render errorSummary()}
			{/if}

			<Heading level={1} variant="page" text={title} />

			{@render content()}
		</stack-l>
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}
	}
</style>

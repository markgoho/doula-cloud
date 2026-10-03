<script lang="ts">
	/*
	 * One page per Practice-named section (#466, ADR-0017's
	 * Practice-defined layer).
	 *
	 * The sections are computed, never written down: `intakeJourney.ts`
	 * splits the Practice's Client Field Template on its `section_header`
	 * entries, and fields that sit before the first header become a
	 * section headed with the Practice's own name. A Practice that has
	 * added nothing has no sections, so this route is never reached.
	 *
	 * A `legend` question: the section's name is the page's <h1> and the
	 * <fieldset> groups every field asked under it. Every field in it is
	 * optional, so the legend says so (#1610), after the Practice's own
	 * words.
	 */
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import ClientFieldAnswers from '#lib/components/organisms/ClientFieldAnswers.svelte';
	import { sectionStepId } from '#lib/intakeJourney.js';
	import JourneyQuestion from './JourneyQuestion.svelte';
	import type { QuestionJourney } from './questionJourney.js';

	let { journey }: { journey: QuestionJourney } = $props();

	const index = $derived(Number(page.params.sectionIndex));
	const section = $derived(journey.sections[index]);

	// A Practice can archive its last field, or rename a section, between
	// one visit and the next -- and a bookmark outlives either. An index
	// with no section behind it is the end of the sequence rather than an
	// error page: the check page is where it would have led anyway. An
	// effect rather than an onMount, because SvelteKit keeps one component
	// across a change of `sectionIndex` and this has to follow it.
	$effect(() => {
		if (section === undefined && journey.isReady) {
			void goto(`${journey.basePath}/check`);
		}
	});
</script>

{#if section}
	<JourneyQuestion
		{journey}
		stepId={sectionStepId(index)}
		question={{ as: 'legend', text: `${section.heading} (optional)` }}
		hint="These are the questions this Practice asks. Every one of them can be left for later."
	>
		{#snippet controls()}
			<ClientFieldAnswers
				fields={section.fields}
				values={journey.draft.answers.fieldValues}
				onChange={(fieldId, value) => journey.draft.setFieldValue(fieldId, value)}
				idPrefix="intake-field-{index}"
			/>
		{/snippet}
	</JourneyQuestion>
{/if}

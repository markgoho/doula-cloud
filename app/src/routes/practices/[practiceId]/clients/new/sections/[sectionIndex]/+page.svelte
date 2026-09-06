<script lang="ts">
	/*
	 * One page per Practice-named section (#466, ADR-0017's
	 * Practice-defined layer).
	 *
	 * The sections are computed, never written down: `intakeJourney.ts`
	 * splits the Practice's Client Field Template on its `section_header`
	 * entries, and fields that sit before the first header become a
	 * section headed with the Practice's own name. A Practice that has
	 * added nothing has no sections, so this route is never reached and
	 * the journey is five steps rather than six with an empty one.
	 *
	 * A `legend` question: the section's name is the page's <h1> and the
	 * <fieldset> groups every field asked under it.
	 */
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import ClientFieldAnswers from '#lib/components/organisms/ClientFieldAnswers.svelte';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import { intakeFlow } from '#lib/intakeFlow.svelte.js';
	import { sectionStepId } from '#lib/intakeJourney.js';
	import IntakeQuestion from '../../IntakeQuestion.svelte';
	import { basePath } from '../../intake.js';

	const practiceId = $derived(page.params.practiceId ?? '');
	const index = $derived(Number(page.params.sectionIndex));
	const section = $derived(intakeFlow.sections[index]);

	// A Practice can archive its last field, or rename a section, between
	// one visit and the next -- and a bookmark outlives either. An index
	// with no section behind it is the end of the sequence rather than an
	// error page: the summary is where it would have led anyway. An
	// effect rather than an onMount, because SvelteKit keeps one component
	// across a change of `sectionIndex` and this has to follow it.
	$effect(() => {
		if (section === undefined && intakeFlow.status === 'ready') {
			void goto(`${basePath(practiceId)}/check`);
		}
	});
</script>

{#if section}
	<IntakeQuestion
		stepId={sectionStepId(index)}
		question={{ as: 'legend', text: section.heading }}
		hint="These are the questions this Practice asks. Every one of them can be left for later."
	>
		{#snippet controls()}
			<ClientFieldAnswers
				fields={section.fields}
				values={intakeDraft.answers.fieldValues}
				onChange={(fieldId, value) => intakeDraft.setFieldValue(fieldId, value)}
				idPrefix="intake-field-{index}"
			/>
		{/snippet}
	</IntakeQuestion>
{/if}

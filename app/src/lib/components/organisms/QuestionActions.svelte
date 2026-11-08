<script lang="ts" generics="F extends Listed">
	/*
	 * The buttons of a Practice's own question page (#2193): Save, and the
	 * retire button that opens `QuestionFields`' confirmation. A question not
	 * yet saved has nothing to retire. Drawn in `QuestionPage`'s actions
	 * region, inside the route's `<form>`.
	 */
	import type { Listed, PracticeQuestion } from '#lib/practiceQuestion.svelte.js';
	import Button from '../atoms/Button.svelte';

	interface Properties {
		question: PracticeQuestion<F>;
	}

	let { question }: Properties = $props();
</script>

<Button type="submit" label="Save question" loading={question.submission.isSubmitting} />
{#if !question.isNew}
	<Button
		label={`${question.retireVerb} question`}
		variant="link"
		onClick={() => (question.isConfirmingRetire = true)}
	/>
{/if}

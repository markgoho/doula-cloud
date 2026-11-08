<script lang="ts" generics="F extends Listed">
	/*
	 * The controls of a Practice's own question page and the confirmation
	 * that retires it (#2193): `PracticeQuestionForm` bound to the page's
	 * `PracticeQuestion` controller, and the dialog its retire button opens.
	 * The route owns the `<form>` and the `QuestionPage` around it, so the
	 * dialog's own buttons sit in the form without submitting it.
	 */
	import type { Listed, PracticeQuestion } from '#lib/practiceQuestion.svelte.js';
	import ConfirmDialog from '../molecules/ConfirmDialog.svelte';
	import PracticeQuestionForm from './PracticeQuestionForm.svelte';

	interface Properties {
		question: PracticeQuestion<F>;
	}

	let { question }: Properties = $props();
</script>

<PracticeQuestionForm
	name={question.name}
	type={question.type}
	label={question.label}
	optionsText={question.optionsText}
	position={question.position}
	positionCount={question.positionCount}
	isKindLocked={question.isKindLocked}
	errorFor={(target) => question.submission.errorFor(target)}
	onLabelInput={(value) => (question.label = value)}
	onTypeChange={(value) => (question.type = value)}
	onOptionsInput={(value) => (question.optionsText = value)}
	onPositionChange={(value) => (question.position = value)}
/>

<ConfirmDialog
	bind:open={question.isConfirmingRetire}
	title={`${question.retireVerb} "${question.shownLabel}"?`}
	consequence={question.retireConsequence}
	confirmLabel={`${question.retireVerb} question`}
	onConfirm={() => question.retire()}
/>

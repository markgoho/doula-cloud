<script lang="ts">
	/*
	 * One of the Practice's own sections, in her details journey (#1610).
	 * The question itself is shared with intake -- see
	 * `questions/questionJourney.ts`.
	 */
	import SectionQuestion from '../../../../questions/SectionQuestion.svelte';
	import { detailsQuestions } from '../../details.js';
</script>

<SectionQuestion journey={detailsQuestions} />

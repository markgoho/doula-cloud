<script lang="ts">
	/*
	 * One of intake's Practice-named sections (#466). The question itself
	 * is shared with a Client's details journey (#1610) -- see
	 * `questions/questionJourney.ts`.
	 */
	import SectionQuestion from '../../../questions/SectionQuestion.svelte';
	import { intakeQuestions } from '../../intake.js';
</script>

<SectionQuestion journey={intakeQuestions} />

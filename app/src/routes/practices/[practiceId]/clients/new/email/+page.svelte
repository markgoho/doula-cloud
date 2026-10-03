<script lang="ts">
	/*
	 * Intake's email address question (#466). The question itself is
	 * shared with a Client's details journey (#1610) -- see
	 * `questions/questionJourney.ts`.
	 */
	import EmailQuestion from '../../questions/EmailQuestion.svelte';
	import { intakeQuestions } from '../intake.js';
</script>

<EmailQuestion journey={intakeQuestions} />

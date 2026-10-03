<script lang="ts">
	/*
	 * Intake's date of birth question (#466). The question itself is
	 * shared with a Client's details journey (#1610) -- see
	 * `questions/questionJourney.ts`.
	 */
	import DateOfBirthQuestion from '../../questions/DateOfBirthQuestion.svelte';
	import { intakeQuestions } from '../intake.js';
</script>

<DateOfBirthQuestion journey={intakeQuestions} />

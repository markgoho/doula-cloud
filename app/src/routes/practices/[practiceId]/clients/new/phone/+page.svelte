<script lang="ts">
	/*
	 * Intake's phone number question (#466). The question itself is
	 * shared with a Client's details journey (#1610) -- see
	 * `questions/questionJourney.ts`.
	 */
	import PhoneQuestion from '../../questions/PhoneQuestion.svelte';
	import { intakeQuestions } from '../intake.js';
</script>

<PhoneQuestion journey={intakeQuestions} />

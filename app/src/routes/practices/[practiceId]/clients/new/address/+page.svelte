<script lang="ts">
	/*
	 * Intake's address question (#466). The question itself is shared
	 * with a Client's details journey (#1610) -- see
	 * `questions/questionJourney.ts`.
	 */
	import AddressQuestion from '../../questions/AddressQuestion.svelte';
	import { intakeQuestions } from '../intake.js';
</script>

<AddressQuestion journey={intakeQuestions} />

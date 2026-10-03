<script lang="ts">
	/*
	 * Her date of birth, the first question of her details journey
	 * (#1610). The question itself is shared with intake -- see
	 * `questions/questionJourney.ts`.
	 */
	import DateOfBirthQuestion from '../../../questions/DateOfBirthQuestion.svelte';
	import { detailsQuestions } from '../details.js';
</script>

<DateOfBirthQuestion journey={detailsQuestions} />

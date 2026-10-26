<script lang="ts">
	/*
	 * Her name, in her details journey (#2048). The question itself is
	 * shared -- see `questions/questionJourney.ts`.
	 */
	import NameQuestion from '../../../questions/NameQuestion.svelte';
	import { detailsQuestions } from '../details.js';
</script>

<NameQuestion journey={detailsQuestions} />

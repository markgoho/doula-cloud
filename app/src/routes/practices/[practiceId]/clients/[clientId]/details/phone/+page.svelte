<script lang="ts">
	/*
	 * Her phone number, in her details journey (#1610). The question
	 * itself is shared with intake -- see `questions/questionJourney.ts`.
	 */
	import PhoneQuestion from '../../../questions/PhoneQuestion.svelte';
	import { detailsQuestions } from '../details.js';
</script>

<PhoneQuestion journey={detailsQuestions} />

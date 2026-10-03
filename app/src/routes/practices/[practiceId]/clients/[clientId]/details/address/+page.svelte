<script lang="ts">
	/*
	 * Her address, in her details journey (#1610). The question itself is
	 * shared with intake -- see `questions/questionJourney.ts`.
	 */
	import AddressQuestion from '../../../questions/AddressQuestion.svelte';
	import { detailsQuestions } from '../details.js';
</script>

<AddressQuestion journey={detailsQuestions} />

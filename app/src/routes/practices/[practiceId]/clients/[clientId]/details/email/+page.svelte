<script lang="ts">
	/*
	 * Her email address, in her details journey (#1610). The question
	 * itself is shared with intake -- see `questions/questionJourney.ts`.
	 */
	import { clientDetails } from '#lib/clientDetailsFlow.svelte.js';
	import EmailQuestion from '../../../questions/EmailQuestion.svelte';
	import { detailsQuestions } from '../details.js';
</script>

<EmailQuestion journey={detailsQuestions} emailOnFile={clientDetails.record?.email} />

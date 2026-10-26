<script lang="ts">
	/*
	 * The Practice's name, on its own page (#2049). Optional (#1536): the
	 * hint says both things before she types, what an empty box does and
	 * where Clients see the name (#1493), so nothing stops Continue here.
	 */
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import SignupQuestion from '../SignupQuestion.svelte';
	import { FIELD_IDS, signupDraft } from '../signupJourney.svelte.js';
</script>

<SignupQuestion
	stepId="practice"
	question={{ as: 'label', text: 'What is your Practice called? (optional)', for: FIELD_IDS.practiceName }}
	hint="Clients see it on Contracts and card statements. Leave it empty to use your own name."
>
	{#snippet controls({ describedBy, errorFor })}
		<TextInput
			id={FIELD_IDS.practiceName}
			{describedBy}
			invalid={Boolean(errorFor(FIELD_IDS.practiceName))}
			value={signupDraft.answers.practiceName}
			onInput={(value) => signupDraft.update({ practiceName: value })}
			autocomplete="organization"
		/>
	{/snippet}
</SignupQuestion>

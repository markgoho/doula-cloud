<script lang="ts">
	import ChoiceGroup from '#lib/components/molecules/ChoiceGroup.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Choice group</h1>

	<p>
		The question around a set of choices: a legend, an optional hint, and one refusal for the whole
		group. The radio group and the checkbox group both draw their options inside it; their own
		pages show it with options in it.
	</p>

	<ChoiceGroup
		name="style-guide-choice-group"
		legend="How should we reach you?"
		hint="Choose the way you read first."
		error="Choose a way to reach you."
	>
		<p>The choices go here.</p>
	</ChoiceGroup>
</stack-l>

<script lang="ts">
	// PROTOTYPE (#371) -- throwaway. The two axes that are not the
	// variant: which case is being typed, and how much the prompt is
	// allowed to say about an existing Client.
	import { scenarios, disclosureLabels, type Scenario, type Disclosure } from './fixtures.js';

	interface Properties {
		scenario: Scenario;
		disclosure: Disclosure;
		onScenario: (s: Scenario) => void;
		onDisclosure: (d: Disclosure) => void;
		blurb: string;
	}

	let { scenario, disclosure, onScenario, onDisclosure, blurb }: Properties = $props();
	const levels: Disclosure[] = ['confirm-only', 'named', 'full'];
</script>

<div class="harness">
	<p class="blurb">{blurb}</p>

	<fieldset>
		<legend>Case being typed</legend>
		{#each scenarios as s (s.key)}
			<button
				type="button"
				class:on={s.key === scenario.key}
				onclick={() => onScenario(s)}>{s.label}</button
			>
		{/each}
		<p class="trap">{scenario.trap}</p>
	</fieldset>

	<fieldset>
		<legend>What the prompt may print</legend>
		{#each levels as level (level)}
			<button type="button" class:on={level === disclosure} onclick={() => onDisclosure(level)}
				>{disclosureLabels[level]}</button
			>
		{/each}
	</fieldset>
</div>

<style>
	.harness {
		border: 2px dashed #b45309;
		background: #fffbeb;
		color: #431407;
		padding: 0.75rem 1rem 1rem;
		margin-block-end: 2rem;
		border-radius: 0.5rem;
		font-family: ui-monospace, monospace;
		font-size: 0.8125rem;
	}

	.blurb {
		margin: 0 0 0.75rem;
		font-weight: 700;
	}

	fieldset {
		border: 0;
		padding: 0;
		margin: 0 0 0.5rem;
		display: flex;
		flex-wrap: wrap;
		gap: 0.375rem;
		align-items: center;
	}

	legend {
		float: inline-start;
		margin-inline-end: 0.5rem;
		opacity: 0.7;
	}

	button {
		border: 1px solid #b45309;
		background: transparent;
		color: inherit;
		border-radius: 999px;
		padding: 0.25rem 0.625rem;
		font: inherit;
		cursor: pointer;
	}

	button.on {
		background: #b45309;
		color: #fff;
	}

	.trap {
		flex-basis: 100%;
		margin: 0.25rem 0 0;
		font-style: italic;
		opacity: 0.85;
	}
</style>

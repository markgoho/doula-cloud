<script lang="ts">
	// PROTOTYPE (#371) VARIANT A -- Match at intake.
	// Entry point: today's "Add a Client". Nothing changes about how she
	// starts; the match arrives as an inline banner while she types, and
	// the Credit is confirmed in the submit label, not in a step of its own.
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import {
		findMatches,
		matchHeadline,
		matchDetail,
		hasLiveEngagement,
		displayName,
		type Disclosure,
		type Scenario
	} from './fixtures.js';

	interface Properties {
		scenario: Scenario;
		disclosure: Disclosure;
	}

	let { scenario, disclosure }: Properties = $props();

	let typed = $state({ ...scenario.typed });
	let dismissed = $state<string[]>([]);
	let reusing = $state<string | undefined>();
	let outcome = $state('');

	// Re-seed the form whenever the harness switches case.
	$effect(() => {
		typed = { ...scenario.typed };
		dismissed = [];
		reusing = undefined;
		outcome = '';
	});

	const matches = $derived(findMatches(typed).filter((m) => !dismissed.includes(m.client.id)));
	const reused = $derived(matches.find((m) => m.client.id === reusing));

	function submit(event: SubmitEvent) {
		event.preventDefault();
		outcome = reused
			? `Reused ${displayName(reused.client)} (${reused.client.id}). New Engagement created. 1 Credit spent.`
			: `Created a new Client for ${typed.givenName} ${typed.familyName}. New Engagement created. 1 Credit spent.`;
	}
</script>

<Heading level={1} text="Add a Client" />

<form onsubmit={submit}>
	<LabeledField label="First name">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput {id} {describedBy} {invalid} value={typed.givenName} onInput={(v) => (typed.givenName = v)} required />
		{/snippet}
	</LabeledField>
	<LabeledField label="Last name">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput {id} {describedBy} {invalid} value={typed.familyName} onInput={(v) => (typed.familyName = v)} />
		{/snippet}
	</LabeledField>
	<LabeledField label="Email">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput {id} {describedBy} {invalid} type="email" value={typed.email} onInput={(v) => (typed.email = v)} />
		{/snippet}
	</LabeledField>
	<LabeledField label="Phone">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput {id} {describedBy} {invalid} value={typed.phone} onInput={(v) => (typed.phone = v)} />
		{/snippet}
	</LabeledField>

	{#each matches as match (match.client.id)}
		<aside class="match" class:chosen={match.client.id === reusing}>
			<p class="headline">{matchHeadline(match, disclosure)}</p>
			{#if match.confidence === 'near'}
				<Badge label={`Near match — same ${match.on}, different email`} variant="warning" />
			{/if}
			{#each matchDetail(match, disclosure) as line (line)}
				<Text text={line} size="sm" muted />
			{/each}
			{#if disclosure !== 'confirm-only' && hasLiveEngagement(match.client)}
				<Badge label="She already has a live Engagement" variant="warning" />
			{/if}
			<div class="actions">
				{#if match.client.id === reusing}
					<Text text="Using her record. The new Engagement will be added to it." size="sm" />
					<Button label="Undo" variant="secondary" size="sm" onClick={() => (reusing = undefined)} />
				{:else}
					<Button label="Use her record" size="sm" onClick={() => (reusing = match.client.id)} />
					<Button
						label="No — different person"
						variant="secondary"
						size="sm"
						onClick={() => (dismissed = [...dismissed, match.client.id])}
					/>
				{/if}
			</div>
		</aside>
	{/each}

	<Button type="submit" label={reused ? 'Add Engagement — uses 1 Credit' : 'Add Client — uses 1 Credit'} />
</form>

{#if outcome}
	<pre class="outcome">{outcome}</pre>
{/if}

<style>
	form {
		display: grid;
		gap: var(--space-3, 0.75rem);
		max-width: 32rem;
	}

	.match {
		border: 1px solid var(--color-border, #d4d4d8);
		border-inline-start: 4px solid #b45309;
		border-radius: 0.375rem;
		padding: 0.75rem;
		display: grid;
		gap: 0.375rem;
		justify-items: start;
	}

	.match.chosen {
		border-inline-start-color: #15803d;
		background: #f0fdf4;
	}

	.headline {
		margin: 0;
		font-weight: 600;
	}

	.actions {
		display: flex;
		gap: 0.5rem;
		align-items: center;
		flex-wrap: wrap;
	}

	.outcome {
		margin-block-start: 1.5rem;
		padding: 0.75rem;
		background: #111;
		color: #a7f3d0;
		border-radius: 0.375rem;
		white-space: pre-wrap;
		font-size: 0.8125rem;
	}
</style>

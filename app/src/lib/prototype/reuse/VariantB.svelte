<script lang="ts">
	// PROTOTYPE (#371) VARIANT B -- Search first.
	// Entry point: "Start new work" opens a search, not a form. Reuse is
	// the default path and intake is the fallback the search sends you to
	// when nobody matches. The Credit gets a step of its own, because
	// spending it is the irreversible act.
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import {
		search,
		matchDetail,
		hasLiveEngagement,
		displayName,
		type StubClient,
		type Disclosure,
		type Scenario
	} from './fixtures.js';

	interface Properties {
		scenario: Scenario;
		disclosure: Disclosure;
	}

	let { scenario, disclosure }: Properties = $props();

	let step = $state<'search' | 'confirm' | 'intake' | 'done'>('search');
	let query = $state('');
	let picked = $state<StubClient | undefined>();
	let typed = $state({ ...scenario.typed });
	let outcome = $state('');

	$effect(() => {
		query = scenario.typed.email;
		typed = { ...scenario.typed };
		step = 'search';
		picked = undefined;
		outcome = '';
	});

	const results = $derived(search(query));

	function pick(client: StubClient) {
		picked = client;
		step = 'confirm';
	}
</script>

{#if step === 'search'}
	<Heading level={1} text="Who are you starting work with?" />
	<Text text="Search by name, email, or phone. Add her as a new Client only if she is not here." muted />
	<div class="search">
		<LabeledField label="Search this Practice's Clients">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput {id} {describedBy} {invalid} value={query} onInput={(v) => (query = v)} />
			{/snippet}
		</LabeledField>
	</div>

	{#if query.trim().length >= 2}
		<ul class="results">
			{#each results as client (client.id)}
				<li>
					<div>
						<p class="name">
							{disclosure === 'confirm-only' ? 'An existing Client' : displayName(client)}
						</p>
						{#each matchDetail({ client, on: 'name', confidence: 'exact' }, disclosure) as line (line)}
							<Text text={line} size="sm" muted />
						{/each}
						{#if hasLiveEngagement(client)}
							<Badge label="Live Engagement" variant="warning" />
						{/if}
					</div>
					<Button label="Start new work with her" size="sm" onClick={() => pick(client)} />
				</li>
			{:else}
				<li class="empty"><Text text="Nobody at this Practice matches that." muted /></li>
			{/each}
		</ul>
		<Button
			label="She is not here — add her as a new Client"
			variant="secondary"
			onClick={() => (step = 'intake')}
		/>
	{/if}
{/if}

{#if step === 'confirm' && picked}
	<Heading level={1} text="Start new work" />
	<div class="card">
		<p class="name">{displayName(picked)}</p>
		{#each matchDetail({ client: picked, on: 'name', confidence: 'exact' }, 'full') as line (line)}
			<Text text={line} size="sm" muted />
		{/each}
		{#if hasLiveEngagement(picked)}
			<Badge label="She already has a live Engagement — is this a second, separate piece of work?" variant="warning" />
		{/if}
	</div>
	<Text text="This adds a new Engagement to her existing record and uses 1 Credit. It does not create a second Client." />
	<div class="actions">
		<Button
			label="Yes — add an Engagement, use 1 Credit"
			onClick={() => {
				outcome = `Reused ${displayName(picked!)} (${picked!.id}). New Engagement created. 1 Credit spent.`;
				step = 'done';
			}}
		/>
		<Button label="No — this is a different person" variant="secondary" onClick={() => (step = 'intake')} />
	</div>
{/if}

{#if step === 'intake'}
	<Heading level={1} text="Add a Client" />
	<form
		onsubmit={(event) => {
			event.preventDefault();
			outcome = `Created a new Client for ${typed.givenName} ${typed.familyName}. New Engagement created. 1 Credit spent.`;
			step = 'done';
		}}
	>
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
		<Button type="submit" label="Add Client — uses 1 Credit" />
		<Button label="Back to search" variant="secondary" onClick={() => (step = 'search')} />
	</form>
{/if}

{#if step === 'done'}
	<pre class="outcome">{outcome}</pre>
	<Button label="Start again" variant="secondary" onClick={() => (step = 'search')} />
{/if}

<style>
	.search,
	form {
		display: grid;
		gap: var(--space-3, 0.75rem);
		max-width: 32rem;
		margin-block: 1rem;
	}

	.results {
		list-style: none;
		padding: 0;
		margin: 0 0 1rem;
		display: grid;
		gap: 0.5rem;
		max-width: 40rem;
	}

	.results li {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 1rem;
		border: 1px solid var(--color-border, #d4d4d8);
		border-radius: 0.375rem;
		padding: 0.75rem;
	}

	.results li.empty {
		justify-content: flex-start;
		border-style: dashed;
	}

	.card {
		border: 1px solid var(--color-border, #d4d4d8);
		border-radius: 0.375rem;
		padding: 0.75rem;
		max-width: 32rem;
		margin-block: 1rem;
		display: grid;
		gap: 0.25rem;
		justify-items: start;
	}

	.name {
		margin: 0;
		font-weight: 600;
	}

	.actions {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
		margin-block-start: 1rem;
	}

	.outcome {
		margin-block: 1.5rem;
		padding: 0.75rem;
		background: #111;
		color: #a7f3d0;
		border-radius: 0.375rem;
		white-space: pre-wrap;
		font-size: 0.8125rem;
	}
</style>

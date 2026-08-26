<script lang="ts">
	// PROTOTYPE (#371) VARIANT D -- One box, then a review sheet.
	// Entry point: today's "Add a Client", but the first field is a
	// find-or-create box: typing searches the Practice's Clients and
	// offers them, and ignoring the offers is how you create a new one.
	// No separate search screen, no banner. The one confirmation is a
	// review sheet at the end, which is also where the Credit is named --
	// so reuse and spend are confirmed by the same click.
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

	let who = $state('');
	let typed = $state({ ...scenario.typed });
	let chosen = $state<StubClient | undefined>();
	let ignored = $state(false);
	let reviewing = $state(false);
	let outcome = $state('');

	$effect(() => {
		who = `${scenario.typed.givenName} ${scenario.typed.familyName}`;
		typed = { ...scenario.typed };
		chosen = undefined;
		ignored = false;
		reviewing = false;
		outcome = '';
	});

	const suggestions = $derived(chosen || ignored ? [] : search(who));

	function choose(client: StubClient) {
		chosen = client;
		who = displayName(client);
		typed = {
			givenName: client.givenName,
			familyName: client.familyName,
			email: client.email ?? '',
			phone: client.phone ?? ''
		};
	}
</script>

<Heading level={1} text="Start new work" />

{#if !reviewing}
	<form
		onsubmit={(event) => {
			event.preventDefault();
			reviewing = true;
		}}
	>
		<LabeledField label="Who is she?">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					value={who}
					onInput={(v) => ((who = v), (chosen = undefined), (ignored = false))}
					required
				/>
			{/snippet}
		</LabeledField>

		{#if suggestions.length > 0}
			<ul class="suggest">
				{#each suggestions as client (client.id)}
					<li>
						<button type="button" onclick={() => choose(client)}>
							<span class="name"
								>{disclosure === 'confirm-only' ? 'An existing Client' : displayName(client)}</span
							>
							{#each matchDetail({ client, on: 'name', confidence: 'exact' }, disclosure) as line (line)}
								<span class="meta">{line}</span>
							{/each}
						</button>
						{#if hasLiveEngagement(client)}<Badge label="Live Engagement" variant="warning" />{/if}
					</li>
				{/each}
				<li class="dismiss">
					<Button
						label="None of these — she is new"
						variant="secondary"
						size="sm"
						onClick={() => (ignored = true)}
					/>
				</li>
			</ul>
		{/if}

		{#if chosen}
			<div class="chosen">
				<Text text={`Adding to ${displayName(chosen)}'s record. Her details below are hers, not new entries.`} size="sm" />
				<Button label="Not her" variant="secondary" size="sm" onClick={() => ((chosen = undefined), (ignored = true))} />
			</div>
		{/if}

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

		<Button type="submit" label="Review" />
	</form>
{:else}
	<div class="sheet">
		<Heading level={2} text="Before this is saved" />
		{#if chosen}
			<Text text={`Reusing ${displayName(chosen)} — no second record is created.`} />
			{#if hasLiveEngagement(chosen)}
				<Badge label="She already has a live Engagement" variant="warning" />
			{/if}
		{:else}
			<Text text={`Creating a new Client: ${typed.givenName} ${typed.familyName}.`} />
		{/if}
		<Text text="A new Engagement is created and 1 Credit is used. Credits are not refunded." />
		<div class="actions">
			<Button
				label="Save — use 1 Credit"
				onClick={() => {
					outcome = chosen
						? `Reused ${displayName(chosen)} (${chosen.id}). New Engagement created. 1 Credit spent.`
						: `Created a new Client for ${typed.givenName} ${typed.familyName}. New Engagement created. 1 Credit spent.`;
					reviewing = false;
				}}
			/>
			<Button label="Go back" variant="secondary" onClick={() => (reviewing = false)} />
		</div>
	</div>
{/if}

{#if outcome}
	<pre class="outcome">{outcome}</pre>
{/if}

<style>
	form {
		display: grid;
		gap: var(--space-3, 0.75rem);
		max-width: 32rem;
	}

	.suggest {
		list-style: none;
		margin: -0.5rem 0 0;
		padding: 0.25rem;
		border: 1px solid var(--color-border, #d4d4d8);
		border-radius: 0.375rem;
		display: grid;
		gap: 0.25rem;
	}

	.suggest li {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.suggest button {
		flex: 1;
		display: grid;
		gap: 0.125rem;
		text-align: start;
		border: 0;
		background: none;
		padding: 0.5rem;
		border-radius: 0.25rem;
		font: inherit;
		cursor: pointer;
	}

	.suggest button:hover {
		background: var(--color-surface-muted, #f4f4f5);
	}

	.name {
		font-weight: 600;
	}

	.meta {
		font-size: 0.8125rem;
		opacity: 0.7;
	}

	.dismiss {
		justify-content: flex-start;
		padding: 0.25rem 0.5rem;
	}

	.chosen,
	.sheet {
		border: 1px solid var(--color-border, #d4d4d8);
		border-radius: 0.375rem;
		padding: 0.75rem;
		display: grid;
		gap: 0.5rem;
		justify-items: start;
		max-width: 32rem;
	}

	.actions {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
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

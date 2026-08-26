<script lang="ts">
	// PROTOTYPE (#371) VARIANT C -- Start new work from her page.
	// Entry point: the Clients list. Intake is only ever for a genuinely
	// new person; reuse is an action on the Client's own record, where
	// her history is already in front of you. The intake form defends the
	// rule with a hard stop, not a suggestion.
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import {
		clients,
		findMatches,
		matchHeadline,
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

	let step = $state<'list' | 'detail' | 'intake' | 'done'>('list');
	let open = $state<StubClient | undefined>();
	let confirming = $state(false);
	let typed = $state({ ...scenario.typed });
	let overrode = $state(false);
	let outcome = $state('');

	$effect(() => {
		typed = { ...scenario.typed };
		step = 'list';
		open = undefined;
		confirming = false;
		overrode = false;
		outcome = '';
	});

	const blockers = $derived(
		overrode ? [] : findMatches(typed).filter((m) => m.confidence === 'exact')
	);
</script>

{#if step === 'list'}
	<Heading level={1} text="Clients" />
	<ul class="results">
		{#each clients as client (client.id)}
			<li>
				<button type="button" onclick={() => ((open = client), (step = 'detail'))}
					>{displayName(client)}</button
				>
				{#if hasLiveEngagement(client)}<Badge label="Live" variant="info" />{/if}
			</li>
		{/each}
	</ul>
	<Button label="Add a Client" onClick={() => (step = 'intake')} />
{/if}

{#if step === 'detail' && open}
	<Heading level={1} text={displayName(open)} />
	<div class="card">
		<Text text={open.email ?? 'No email on file'} size="sm" muted />
		<Text text={open.phone ?? 'No phone on file'} size="sm" muted />
		<Text text={open.locality ?? ''} size="sm" muted />
	</div>

	<Heading level={2} text="Her work with this Practice" />
	<ul class="history">
		{#each open.engagements as engagement (engagement.span)}
			<li>
				<Text text={`${engagement.kind === 'birth' ? 'Birth' : 'Postpartum'} — ${engagement.span}`} />
				<Badge label={engagement.status} variant={engagement.status === 'closed' ? 'neutral' : 'info'} />
			</li>
		{/each}
	</ul>

	{#if confirming}
		<div class="confirm">
			<Text text={`Start a new Engagement with ${displayName(open)}. This uses 1 Credit.`} />
			{#if hasLiveEngagement(open)}
				<Notice
					variant="info"
					message="She already has a live Engagement. A second one runs alongside it — check this is separate work, not a correction to the one she has."
				/>
			{/if}
			<div class="actions">
				<Button
					label="Yes — use 1 Credit"
					onClick={() => {
						outcome = `Reused ${displayName(open!)} (${open!.id}). New Engagement created. 1 Credit spent.`;
						step = 'done';
					}}
				/>
				<Button label="Cancel" variant="secondary" onClick={() => (confirming = false)} />
			</div>
		</div>
	{:else}
		<div class="actions">
			<Button label={`Start new work with ${open.givenName}`} onClick={() => (confirming = true)} />
			<Button label="Edit her details" variant="secondary" />
			<Button label="Back to Clients" variant="secondary" onClick={() => (step = 'list')} />
		</div>
	{/if}
{/if}

{#if step === 'intake'}
	<Heading level={1} text="Add a Client" />
	<Text text="Only for someone this Practice has never worked with. To start new work with an existing Client, open her record." muted />
	<form
		onsubmit={(event) => {
			event.preventDefault();
			if (blockers.length > 0) return;
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

		{#each blockers as blocker (blocker.client.id)}
			<Notice variant="error" message={matchHeadline(blocker, disclosure)} />
			<div class="actions">
				<Button
					label="Open her record"
					size="sm"
					onClick={() => ((open = blocker.client), (step = 'detail'))}
				/>
				<Button
					label="This is a different person — carry on"
					variant="secondary"
					size="sm"
					onClick={() => (overrode = true)}
				/>
			</div>
		{/each}

		<Button type="submit" label="Add Client — uses 1 Credit" disabled={blockers.length > 0} />
		<Button label="Back to Clients" variant="secondary" onClick={() => (step = 'list')} />
	</form>
{/if}

{#if step === 'done'}
	<pre class="outcome">{outcome}</pre>
	<Button label="Start again" variant="secondary" onClick={() => (step = 'list')} />
{/if}

<style>
	form {
		display: grid;
		gap: var(--space-3, 0.75rem);
		max-width: 32rem;
		margin-block: 1rem;
	}

	.results,
	.history {
		list-style: none;
		padding: 0;
		margin: 1rem 0;
		display: grid;
		gap: 0.5rem;
		max-width: 40rem;
	}

	.results li,
	.history li {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		border: 1px solid var(--color-border, #d4d4d8);
		border-radius: 0.375rem;
		padding: 0.625rem 0.75rem;
	}

	.results button {
		border: 0;
		background: none;
		padding: 0;
		font: inherit;
		color: var(--color-link, #1d4ed8);
		text-decoration: underline;
		cursor: pointer;
	}

	.card,
	.confirm {
		border: 1px solid var(--color-border, #d4d4d8);
		border-radius: 0.375rem;
		padding: 0.75rem;
		max-width: 32rem;
		margin-block: 1rem;
		display: grid;
		gap: 0.375rem;
		justify-items: start;
	}

	.actions {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
		align-items: center;
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

<script lang="ts">
	// PROTOTYPE (#372) -- throwaway. #371's save-time match prompt, identical in
	// every variant because #371 already settled it. What each variant decides is
	// WHERE this lands in its flow, not what it says.
	import Button from '#lib/components/atoms/Button.svelte';
	import { liveEngagement, type ClientDraft, type ExistingClient } from './fixtures.js';

	interface Properties {
		matches: ExistingClient[];
		typed: ClientDraft;
		onReuse: (existing: ExistingClient) => void;
		onDifferentPerson: () => void;
	}

	let { matches, typed, onReuse, onDifferentPerson }: Properties = $props();

	function differences(existing: ExistingClient): string[] {
		const out: string[] = [];
		if (typed.email && typed.email !== existing.email) {
			out.push(`Email on file is ${existing.email || '—'}; you typed ${typed.email}`);
		}
		if (typed.phone && typed.phone !== existing.phone) {
			out.push(`Phone on file is ${existing.phone || '—'}; you typed ${typed.phone}`);
		}
		if (typed.address_locality && typed.address_locality !== existing.address_locality) {
			out.push(`Area on file is ${existing.address_locality || '—'}; you typed ${typed.address_locality}`);
		}
		return out;
	}
</script>

<section aria-labelledby="match-heading" class="prompt">
	<h2 id="match-heading">This may be someone you already have</h2>
	<p>
		Nothing has been saved. {matches.length === 1 ? 'One record' : `${matches.length} records`} at this
		Practice matched what you typed.
	</p>

	{#each matches as existing (existing.id)}
		{@const live = liveEngagement(existing)}
		{@const changes = differences(existing)}
		<article>
			<h3>{existing.given_name} {existing.family_name}</h3>
			<dl>
				<dt>Goes by</dt>
				<dd>{existing.preferred_name}</dd>
				<dt>Date of birth</dt>
				<dd>{existing.date_of_birth || 'Not recorded'}</dd>
				<dt>Email</dt>
				<dd>{existing.email || 'Not recorded'}</dd>
				<dt>Phone</dt>
				<dd>{existing.phone || 'Not recorded'}</dd>
			</dl>
			{#if existing.engagements.length > 0}
				<ul>
					{#each existing.engagements as engagement (engagement.span)}
						<li>{engagement.kind} · {engagement.span} · {engagement.status}</li>
					{/each}
				</ul>
			{:else}
				<p class="quiet">No work with her yet.</p>
			{/if}

			{#if live}
				<p class="warn">
					She has a live {live.kind} Engagement, {live.span}. Start a second one only if this is
					separate work.
				</p>
			{/if}

			{#if changes.length > 0}
				<p class="quiet">Choosing her record applies these as a recorded edit:</p>
				<ul>
					{#each changes as change (change)}
						<li>{change}</li>
					{/each}
				</ul>
			{/if}

			<Button
				label="This is her — use her record"
				onClick={() => onReuse(existing)}
			/>
		</article>
	{/each}

	<p class="quiet">
		If none of these is the woman you are entering, say so. This is the only way to create a second
		record for the same name.
	</p>
	<Button label="No — a different person" variant="secondary" onClick={onDifferentPerson} />
</section>

<style>
	.prompt {
		border: 2px solid var(--color-text, #111);
		padding: 1rem;
	}

	article {
		border-block-start: 1px solid #ccc;
		padding-block: 0.75rem;
	}

	h3 {
		margin: 0 0 0.5rem;
		font-size: 1rem;
	}

	dl {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 0.125rem 0.75rem;
		margin: 0 0 0.5rem;
		font-size: 0.875rem;
	}

	dt {
		font-weight: 600;
	}

	dd {
		margin: 0;
	}

	ul {
		margin: 0 0 0.75rem;
		padding-inline-start: 1.25rem;
		font-size: 0.875rem;
	}

	.quiet {
		font-size: 0.875rem;
		color: #555;
	}

	.warn {
		font-size: 0.875rem;
		border-inline-start: 3px solid #b45309;
		padding-inline-start: 0.625rem;
	}
</style>

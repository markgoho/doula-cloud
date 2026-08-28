<script lang="ts">
	// PROTOTYPE (#372) -- throwaway. The hub D lands on after the save: GOV.UK's
	// task-list pattern, one short page behind each row. The save already
	// happened, so "Save record" is not an action here -- it is a fact, stated.
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import { fullName, practiceFields, type ClientDraft } from './fixtures.js';

	interface Properties {
		client: ClientDraft;
		custom: Record<string, string | boolean>;
		isSaved: boolean;
		onOpen: (step: 'name' | 'reach' | 'dob' | 'address' | 'practice' | 'request') => void;
		onLeave: () => void;
	}

	let { client, custom, isSaved, onOpen, onLeave }: Properties = $props();

	const knownAs = $derived(client.preferred_name.trim() || client.given_name.trim() || 'this Client');

	const addressKeys: (keyof ClientDraft)[] = [
		'address_line1',
		'address_locality',
		'address_region',
		'address_postal_code'
	];

	const tasks = $derived([
		{
			key: 'reach' as const,
			label: 'Contact details',
			done: Boolean(client.phone.trim() || client.email.trim()),
			hint: [client.phone, client.email].filter(Boolean).join(' · ')
		},
		{
			key: 'dob' as const,
			label: 'Date of birth',
			done: Boolean(client.date_of_birth.trim()),
			hint: client.date_of_birth
		},
		{
			key: 'address' as const,
			label: 'Address',
			done: addressKeys.some((key) => client[key].trim()),
			hint: [client.address_locality, client.address_region].filter(Boolean).join(', ')
		},
		{
			key: 'practice' as const,
			label: 'What this Practice also asks',
			done: practiceFields.some((field) => {
				const value = custom[field.id];
				return typeof value === 'boolean' ? value : Boolean(value);
			}),
			hint: `${practiceFields.length} questions`
		}
	]);

	const outstanding = $derived(tasks.filter((task) => !task.done).length);
</script>

<Heading level={1} text={fullName(client)} />

{#if isSaved}
	<p class="saved">Saved. This record will come up in the search next time, so the same person cannot be typed in twice.</p>
{/if}

<p class="lede">
	{outstanding === 0
		? 'Everything this Practice asks for is recorded.'
		: `${outstanding} of ${tasks.length} things not recorded yet. None of them are needed to keep this record.`}
</p>

<ul class="tasks">
	{#each tasks as task (task.key)}
		<li>
			<span class="label">
				<Button variant="secondary" size="sm" label={task.label} onClick={() => onOpen(task.key)} />
				{#if task.done && task.hint}
					<span class="hint">{task.hint}</span>
				{/if}
			</span>
			<span class="status" class:done={task.done}>{task.done ? 'Recorded' : 'Not started'}</span>
		</li>
	{/each}
</ul>

<section>
	<h2>Work with {knownAs}</h2>
	<p class="quiet">Nothing yet. No Credit has moved.</p>
	<stack-l>
		<Button label={`Ask to start work with ${knownAs}`} onClick={() => onOpen('request')} />
		<Button variant="secondary" label="Leave it here for now" onClick={onLeave} />
	</stack-l>
</section>

<style>
	.saved {
		border-inline-start: 3px solid #15803d;
		padding-inline-start: 0.625rem;
		max-width: 62ch;
	}

	.lede {
		max-width: 62ch;
	}

	.tasks {
		list-style: none;
		margin: 0 0 2rem;
		padding: 0;
		max-width: 34rem;
		border-block-start: 1px solid #ccc;
	}

	.tasks li {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		padding-block: 0.625rem;
		border-block-end: 1px solid #ccc;
	}

	.label {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 0.5rem;
	}

	.hint {
		font-size: 0.8125rem;
		color: #555;
	}

	.status {
		font-size: 0.75rem;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		border: 1px solid #555;
		padding: 0.125rem 0.5rem;
	}

	.status.done {
		border-color: #15803d;
		color: #15803d;
	}

	section h2 {
		font-size: 1.125rem;
	}

	.quiet {
		color: #555;
		font-size: 0.875rem;
	}
</style>

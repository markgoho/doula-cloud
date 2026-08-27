<script lang="ts">
	// PROTOTYPE (#372) -- throwaway. Variant C: minimal create, finish on her
	// page. The form asks for what a staff member has in hand while still on the
	// phone; everything else is filled in later on the Client detail page, which
	// carries a completeness strip and is where "Request Engagement start" lives.
	// Nothing chains -- the Request is a separate visit.
	import Heading from '#lib/components/atoms/Heading.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import PracticeFieldsBlock from './PracticeFieldsBlock.svelte';
	import RequestBlock from './RequestBlock.svelte';
	import MatchPrompt from './MatchPrompt.svelte';
	import {
		matchExisting,
		missingDemanded,
		fullName,
		type ClientDraft,
		type Demands,
		type ExistingClient,
		type RequestDraft
	} from './fixtures.js';

	interface Properties {
		client: ClientDraft;
		request: RequestDraft;
		custom: Record<string, string | boolean>;
		demands: Demands;
		onClient: (patch: Partial<ClientDraft>) => void;
		onRequest: (patch: Partial<RequestDraft>) => void;
		onCustom: (id: string, value: string | boolean) => void;
		onDone: (result: { reused?: string; note: string; withRequest: boolean }) => void;
	}

	let { client, request, custom, demands, onClient, onRequest, onCustom, onDone }: Properties =
		$props();

	let view = $state<'form' | 'record'>('form');
	let matches = $state<ExistingClient[]>([]);
	let errors = $state<string[]>([]);
	let isEditing = $state(false);
	let isAsking = $state(false);

	const restFields: [keyof ClientDraft, string][] = [
		['family_name', 'Last name'],
		['preferred_name', 'What she goes by'],
		['date_of_birth', 'Date of birth'],
		['email', 'Email'],
		['address_line1', 'Street address'],
		['address_line2', 'Apartment, floor'],
		['address_locality', 'Town or city'],
		['address_region', 'State'],
		['address_postal_code', 'ZIP']
	];

	const gaps = $derived(restFields.filter(([key]) => !client[key].trim()));

	function save(event: SubmitEvent) {
		event.preventDefault();
		matches = [];
		errors = missingDemanded(client, demands);
		if (errors.length > 0) return;
		const found = matchExisting(client);
		if (found.length > 0) {
			matches = found;
			return;
		}
		view = 'record';
	}
</script>

{#if matches.length > 0}
	<Heading level={1} text="Add a Client" />
	<MatchPrompt
		{matches}
		typed={client}
		onReuse={() => {
			matches = [];
			view = 'record';
		}}
		onDifferentPerson={() => {
			matches = [];
			view = 'record';
		}}
	/>
{:else if view === 'form'}
	<Heading level={1} text="Add a Client" />
	<p class="lede">
		Enough to find her again later. Everything else waits until you are off the phone — her record
		will tell you what is still missing.
	</p>
	<form onsubmit={save}>
		<stack-l>
			{#if errors.length > 0}
				<Notice variant="error" message={`Still needed: ${errors.join('; ')}.`} />
			{/if}
			<LabeledField label="First name">
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						required
						value={client.given_name}
						onInput={(value) => onClient({ given_name: value })}
					/>
				{/snippet}
			</LabeledField>
			<LabeledField label={demands === 'name-only' ? 'Phone (optional)' : 'Phone'}>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="tel"
						value={client.phone}
						onInput={(value) => onClient({ phone: value })}
					/>
				{/snippet}
			</LabeledField>
			<Button type="submit" label="Save and open her record" />
		</stack-l>
	</form>
{:else}
	<Heading level={1} text={fullName(client)} />
	<p class="lede">Her record. No work has been asked for yet, and no Credit has moved.</p>

	<stack-l>
		{#if gaps.length > 0}
			<div class="gaps">
				<strong>{gaps.length} things not recorded yet</strong>
				<ul>
					{#each gaps as [key, label] (key)}
						<li>{label}</li>
					{/each}
				</ul>
				<Button
					variant="secondary"
					label={isEditing ? 'Hide' : 'Fill these in'}
					onClick={() => (isEditing = !isEditing)}
				/>
			</div>
		{/if}

		{#if isEditing}
			<stack-l>
				{#each restFields as [key, label] (key)}
					<LabeledField label={`${label} (optional)`}>
						{#snippet children({ id, describedBy, invalid })}
							{#if key === 'date_of_birth'}
								<input
									{id}
									type="date"
									aria-describedby={describedBy}
									value={client.date_of_birth}
									oninput={(event) => onClient({ date_of_birth: event.currentTarget.value })}
								/>
							{:else}
								<TextInput
									{id}
									{describedBy}
									{invalid}
									value={client[key]}
									onInput={(value) => onClient({ [key]: value })}
								/>
							{/if}
						{/snippet}
					</LabeledField>
				{/each}
				<PracticeFieldsBlock values={custom} onChange={onCustom} />
				<Button label="Done" onClick={() => (isEditing = false)} />
			</stack-l>
		{/if}

		<section>
			<h2>Work with her</h2>
			<p class="quiet">Nothing yet.</p>
			{#if isAsking}
				<form
					onsubmit={(event) => {
						event.preventDefault();
						errors = request.kind ? [] : ['What kind of work this is'];
						if (errors.length > 0) return;
						onDone({
							note: 'Her record was saved first, on its own. The Request came later, from her page.',
							withRequest: true
						});
					}}
				>
					<stack-l>
						{#if errors.length > 0}
							<Notice variant="error" message={`Still needed: ${errors.join('; ')}.`} />
						{/if}
						<RequestBlock {request} onChange={onRequest} />
						<Button type="submit" label="Send the request" />
					</stack-l>
				</form>
			{:else}
				<Button label="Request Engagement start" onClick={() => (isAsking = true)} />
			{/if}
		</section>

		<Button
			variant="secondary"
			label="Leave it here for now"
			onClick={() =>
				onDone({
					note: 'A Client with no Request. She is findable in the intake search forever, which is the point.',
					withRequest: false
				})}
		/>
	</stack-l>
{/if}

<style>
	.lede {
		max-width: 62ch;
	}

	.gaps {
		border: 1px solid #b45309;
		padding: 0.75rem;
		font-size: 0.875rem;
	}

	.gaps ul {
		margin: 0.375rem 0 0.625rem;
		padding-inline-start: 1.25rem;
		columns: 2;
	}

	section h2 {
		font-size: 1.125rem;
	}

	.quiet {
		color: #555;
		font-size: 0.875rem;
	}
</style>

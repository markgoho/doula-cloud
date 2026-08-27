<script lang="ts">
	// PROTOTYPE (#372) -- throwaway. Variant B: two steps, with a real save
	// between them. Step 1 is the person and it commits on its own; step 2 is the
	// ask. The seam is the one #371 and #393 already cut -- saving her is free,
	// requesting work is not the same act. Order puts date of birth WITH the
	// names, because #371 made it an identity key, not a detail.
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

	let step = $state<1 | 2>(1);
	let matches = $state<ExistingClient[]>([]);
	let errors = $state<string[]>([]);
	let savedAs = $state('');

	const nameFields: [keyof ClientDraft, string][] = [
		['given_name', 'First name'],
		['family_name', 'Last name (optional)'],
		['preferred_name', 'What she goes by (optional)']
	];

	const addressFields: [keyof ClientDraft, string][] = [
		['address_line1', 'Street address'],
		['address_line2', 'Apartment, floor (optional)'],
		['address_locality', 'Town or city'],
		['address_region', 'State'],
		['address_postal_code', 'ZIP']
	];

	function saveClient(event: SubmitEvent) {
		event.preventDefault();
		matches = [];
		errors = missingDemanded(client, demands);
		if (errors.length > 0) return;

		const found = matchExisting(client);
		if (found.length > 0) {
			matches = found;
			return;
		}
		savedAs = fullName(client);
		step = 2;
	}

	function sendRequest(event: SubmitEvent) {
		event.preventDefault();
		errors = request.kind ? [] : ['What kind of work this is'];
		if (errors.length > 0) return;
		onDone({
			note: `${savedAs} was saved at step 1 and stays saved. Step 2 raised one Request against her.`,
			withRequest: true
		});
	}
</script>

{#if matches.length > 0}
	<Heading level={1} text="Add a Client" />
	<MatchPrompt
		{matches}
		typed={client}
		onReuse={(existing) => {
			matches = [];
			savedAs = `${existing.given_name} ${existing.family_name}`;
			step = 2;
		}}
		onDifferentPerson={() => {
			matches = [];
			savedAs = fullName(client);
			step = 2;
		}}
	/>
{:else if step === 1}
	<Heading level={1} text="Add a Client" />
	<p class="lede">
		Step 1 of 2 — who she is. This saves her record on its own. Asking to start paid work is the next
		step, and you can leave before it.
	</p>
	<form onsubmit={saveClient}>
		<stack-l>
			{#if errors.length > 0}
				<Notice variant="error" message={`Still needed: ${errors.join('; ')}.`} />
			{/if}

			<fieldset>
				<legend>Her name, and the date that tells her apart</legend>
				<stack-l>
					{#each nameFields as [key, label] (key)}
						<LabeledField {label}>
							{#snippet children({ id, describedBy, invalid })}
								<TextInput
									{id}
									{describedBy}
									{invalid}
									required={key === 'given_name'}
									placeholder={key === 'preferred_name'
										? client.given_name || 'Same as her first name'
										: undefined}
									value={client[key]}
									onInput={(value) => onClient({ [key]: value })}
								/>
							{/snippet}
						</LabeledField>
					{/each}
					<LabeledField
						label={demands === 'name-reach-dob' ? 'Date of birth' : 'Date of birth (optional)'}
					>
						{#snippet children({ id, describedBy })}
							<input
								{id}
								type="date"
								aria-describedby="{describedBy ?? ''} dob-hint"
								value={client.date_of_birth}
								oninput={(event) => onClient({ date_of_birth: event.currentTarget.value })}
							/>
							<p class="hint" id="dob-hint">
								This is what separates two women with the same name next year.
							</p>
						{/snippet}
					</LabeledField>
				</stack-l>
			</fieldset>

			<fieldset>
				<legend>How to reach her</legend>
				<stack-l>
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
					<LabeledField label={demands === 'name-only' ? 'Email (optional)' : 'Email'}>
						{#snippet children({ id, describedBy, invalid })}
							<TextInput
								{id}
								{describedBy}
								{invalid}
								type="email"
								value={client.email}
								onInput={(value) => onClient({ email: value })}
							/>
						{/snippet}
					</LabeledField>
				</stack-l>
			</fieldset>

			<fieldset>
				<legend>Where she lives</legend>
				<stack-l>
					{#each addressFields as [key, label] (key)}
						<LabeledField label={`${label}${label.includes('optional') ? '' : ' (optional)'}`}>
							{#snippet children({ id, describedBy, invalid })}
								<TextInput
									{id}
									{describedBy}
									{invalid}
									value={client[key]}
									onInput={(value) => onClient({ [key]: value })}
								/>
							{/snippet}
						</LabeledField>
					{/each}
				</stack-l>
			</fieldset>

			<fieldset>
				<legend>What this Practice also asks</legend>
				<PracticeFieldsBlock values={custom} onChange={onCustom} />
			</fieldset>

			<Button type="submit" label="Save her and continue" />
		</stack-l>
	</form>
{:else}
	<Heading level={1} text={`Ask to start work with ${savedAs}`} />
	<p class="lede">
		Step 2 of 2 — the work. {savedAs} is saved either way. This asks an Owner or Admin to approve, and
		the Credit is spent when they do.
	</p>
	<form onsubmit={sendRequest}>
		<stack-l>
			{#if errors.length > 0}
				<Notice variant="error" message={`Still needed: ${errors.join('; ')}.`} />
			{/if}
			<RequestBlock {request} onChange={onRequest} />
			<Button type="submit" label="Send the request" />
			<Button
				variant="secondary"
				label="Not yet — just keep her record"
				onClick={() =>
					onDone({
						note: `${savedAs} is saved with no Request. She appears in the Clients list under "everyone", not under "Clients with work".`,
						withRequest: false
					})}
			/>
		</stack-l>
	</form>
{/if}

<style>
	.lede {
		max-width: 62ch;
	}

	fieldset {
		border: 0;
		padding: 0;
		margin: 0;
	}

	legend {
		font-weight: 600;
		padding: 0;
		margin-block-end: 0.5rem;
	}

	.hint {
		margin: 0;
		font-size: 0.8125rem;
		color: #555;
		max-width: 60ch;
	}
</style>

<script lang="ts">
	// PROTOTYPE (#372) -- throwaway. Variant D, the shape decision 1 converged
	// on: C's fast front door, B's ability to keep going, laid out one-thing-
	// per-page (GOV.UK / Adam Silver) with a task-list hub after the save.
	//
	// The save sits after the third page, not the first, because everything
	// after it crosses #373's edit path -- which blocks and offers only "a
	// different person", never a merge. Deferring a match key past the save
	// makes a duplicate that can no longer be undone.
	import Heading from '#lib/components/atoms/Heading.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import PracticeFieldsBlock from './PracticeFieldsBlock.svelte';
	import RequestBlock from './RequestBlock.svelte';
	import MatchPrompt from './MatchPrompt.svelte';
	import DHub from './DHub.svelte';
	import {
		matchExisting,
		fullName,
		practiceFields,
		type ClientDraft,
		type ExistingClient,
		type RequestDraft
	} from './fixtures.js';

	interface Properties {
		client: ClientDraft;
		request: RequestDraft;
		custom: Record<string, string | boolean>;
		onClient: (patch: Partial<ClientDraft>) => void;
		onRequest: (patch: Partial<RequestDraft>) => void;
		onCustom: (id: string, value: string | boolean) => void;
		onDone: (result: { reused?: string; note: string; withRequest: boolean }) => void;
	}

	let { client, request, custom, onClient, onRequest, onCustom, onDone }: Properties = $props();

	type Step = 'name' | 'reach' | 'dob' | 'hub' | 'address' | 'practice' | 'request';

	// Page 1 is the only screen with no name to use, so it is the only one that
	// could need a pronoun -- and it names the Client instead. Every screen after
	// it addresses her by the name just typed. The product cannot do better than
	// this: pronouns are a Practice-defined field (ADR-0017), which means the
	// product stores them and never reads them.
	const knownAs = $derived(client.preferred_name.trim() || client.given_name.trim() || 'this Client');

	let step = $state<Step>('name');
	let matches = $state<ExistingClient[]>([]);
	let error = $state('');
	let isSaved = $state(false);

	// A client-side page sequence changes the whole page without a navigation,
	// so nothing moves focus and a screen reader never hears the new heading.
	// Focus the heading of each page as it arrives -- and the error summary
	// instead when the submit was refused.
	let pageStart = $state<HTMLElement | undefined>();
	let errorSummary = $state<HTMLElement | undefined>();

	$effect(() => {
		void step;
		void matches.length;
		if (error && errorSummary) {
			errorSummary.focus();
			return;
		}
		pageStart?.focus();
	});

	const addressFields: [keyof ClientDraft, string][] = [
		['address_line1', 'Street address'],
		['address_line2', 'Apartment, floor'],
		['address_locality', 'Town or city'],
		['address_region', 'State'],
		['address_postal_code', 'ZIP']
	];

	function toName(event: SubmitEvent) {
		event.preventDefault();
		error = client.given_name.trim() ? '' : 'Enter a first name.';
		if (!error) step = 'reach';
	}

	function toReach(event: SubmitEvent) {
		event.preventDefault();
		step = 'dob';
	}

	/**
	 * The save. #371's match query runs here, on all four keys.
	 */
	function save(event: SubmitEvent) {
		event.preventDefault();
		const found = matchExisting(client);
		if (found.length > 0) {
			matches = found;
			return;
		}
		isSaved = true;
		step = 'hub';
	}
</script>

{#if matches.length > 0}
	<div bind:this={pageStart} tabindex="-1">
		<Heading level={1} text="Before this is saved" />
	</div>
	<MatchPrompt
		{matches}
		typed={client}
		onReuse={(existing) => {
			matches = [];
			onDone({
				reused: `${existing.given_name} ${existing.family_name}`,
				note: 'Caught at the save. The existing record was kept and what you typed applied as an edit.',
				withRequest: false
			});
		}}
		onDifferentPerson={() => {
			matches = [];
			isSaved = true;
			step = 'hub';
		}}
	/>
{:else if step === 'name'}
	<p class="crumb" id="crumb-1of3">Adding a Client — 1 of 3</p>
	<div bind:this={pageStart} tabindex="-1">
		<Heading level={1} text="What is the Client’s name?" />
	</div>
	<form onsubmit={toName}>
		<stack-l>
			{#if error}
				<div bind:this={errorSummary} tabindex="-1" role="alert">
					<Notice variant="error" message={error} />
				</div>
			{/if}
			<LabeledField label="First name" error={error || undefined}>
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
			<LabeledField label="Last name (optional)">
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						value={client.family_name}
						onInput={(value) => onClient({ family_name: value })}
					/>
				{/snippet}
			</LabeledField>
			<LabeledField label="Goes by (optional)">
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						placeholder={client.given_name || 'Same as the first name'}
						value={client.preferred_name}
						onInput={(value) => onClient({ preferred_name: value })}
					/>
				{/snippet}
			</LabeledField>
			<Button type="submit" label="Add contact details" />
		</stack-l>
	</form>
{:else if step === 'reach'}
	<p class="crumb" id="crumb-2of3">Adding a Client — 2 of 3</p>
	<Button variant="secondary" size="sm" label="Back to the name" onClick={() => (step = 'name')} />
	<div bind:this={pageStart} tabindex="-1">
		<Heading level={1} text={`How do you contact ${knownAs}?`} />
	</div>
	<p class="lede">Either one is enough. The other can be added later.</p>
	<form onsubmit={toReach}>
		<stack-l>
			<LabeledField label="Phone (optional)">
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
			<LabeledField label="Email (optional)">
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
			<Button type="submit" label={`Add ${knownAs}’s date of birth`} />
		</stack-l>
	</form>
{:else if step === 'dob'}
	<p class="crumb" id="crumb-3of3">Adding a Client — 3 of 3</p>
	<Button
		variant="secondary"
		size="sm"
		label="Back to the contact details"
		onClick={() => (step = 'reach')}
	/>
	<div bind:this={pageStart} tabindex="-1">
		<Heading level={1} text={`What is ${knownAs}’s date of birth?`} />
	</div>
	<p class="lede">
		This is what separates two Clients with the same name, next year and the year after. It is the last
		thing asked before the record is saved.
	</p>
	<form onsubmit={save}>
		<stack-l>
			<LabeledField label="Date of birth (optional)">
				{#snippet children({ id, describedBy })}
					<input
						{id}
						type="date"
						aria-describedby={describedBy}
						value={client.date_of_birth}
						oninput={(event) => onClient({ date_of_birth: event.currentTarget.value })}
					/>
				{/snippet}
			</LabeledField>
			<Button type="submit" label={`Save ${fullName(client)}’s record`} />
		</stack-l>
	</form>
{:else if step === 'address'}
	<Button variant="secondary" size="sm" label={`Back to ${knownAs}’s record`} onClick={() => (step = 'hub')} />
	<div bind:this={pageStart} tabindex="-1">
		<Heading level={1} text={`Where does ${knownAs} live?`} />
	</div>
	<p class="lede">Needed before a visit, not before a record.</p>
	<form
		onsubmit={(event) => {
			event.preventDefault();
			step = 'hub';
		}}
	>
		<stack-l>
			{#each addressFields as [key, label] (key)}
				<LabeledField label={`${label} (optional)`}>
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
			<Button type="submit" label="Save this address" />
		</stack-l>
	</form>
{:else if step === 'practice'}
	<Button variant="secondary" size="sm" label={`Back to ${knownAs}’s record`} onClick={() => (step = 'hub')} />
	<div bind:this={pageStart} tabindex="-1">
		<Heading level={1} text="What this Practice also asks" />
	</div>
	<p class="lede">
		{practiceFields.length} questions this Practice added for itself. None of them are required.
	</p>
	<form
		onsubmit={(event) => {
			event.preventDefault();
			step = 'hub';
		}}
	>
		<stack-l>
			<PracticeFieldsBlock values={custom} onChange={onCustom} />
			<Button type="submit" label="Save these answers" />
		</stack-l>
	</form>
{:else if step === 'request'}
	<Button variant="secondary" size="sm" label={`Back to ${knownAs}’s record`} onClick={() => (step = 'hub')} />
	<div bind:this={pageStart} tabindex="-1">
		<Heading level={1} text={`Ask to start work with ${knownAs}`} />
	</div>
	<p class="lede">
		An Owner or Admin approves this. The Credit is spent when they do, not now. The record stays saved
		either way.
	</p>
	<form
		onsubmit={(event) => {
			event.preventDefault();
			error = request.kind ? '' : 'Choose what kind of work this is.';
			if (error) return;
			onDone({
				note: 'The record was saved after page 3, on its own. The Request came from the hub, later and separately.',
				withRequest: true
			});
		}}
	>
		<stack-l>
			{#if error}
				<div bind:this={errorSummary} tabindex="-1" role="alert">
					<Notice variant="error" message={error} />
				</div>
			{/if}
			<RequestBlock {request} onChange={onRequest} />
			<Button type="submit" label="Send this to an Owner or Admin to approve" />
		</stack-l>
	</form>
{:else}
	<DHub
		{client}
		{custom}
		{isSaved}
		onOpen={(next) => (step = next)}
		onLeave={() =>
			onDone({
				note: 'Saved and left. The record is findable in the intake search forever, which is the point. No Request, so it sits outside the "Clients with work" filter.',
				withRequest: false
			})}
	/>
{/if}

<style>
	[tabindex='-1']:focus-visible {
		outline: 2px solid var(--color-text, #111);
		outline-offset: 4px;
	}

	.crumb {
		margin: 0;
		font-size: 0.8125rem;
		color: #555;
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}

	.lede {
		max-width: 62ch;
	}
</style>

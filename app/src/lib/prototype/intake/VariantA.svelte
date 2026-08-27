<script lang="ts">
	// PROTOTYPE (#372) -- throwaway. Variant A: one page, one submit.
	// The Client and the ask live on the same screen; optional detail is behind
	// two disclosures. Order: names, then how to reach her, then date of birth.
	// One act, so the Request is never forgotten -- and never separable.
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

	let matches = $state<ExistingClient[]>([]);
	let errors = $state<string[]>([]);
	let showAddress = $state(false);
	let showPracticeFields = $state(false);

	function submit(event: SubmitEvent) {
		event.preventDefault();
		matches = [];
		errors = missingDemanded(client, demands);
		if (!request.kind) errors.push('What kind of work this is');
		if (errors.length > 0) return;

		const found = matchExisting(client);
		if (found.length > 0) {
			matches = found;
			return;
		}
		onDone({ note: `New Client saved and an Engagement Request raised, in one act.`, withRequest: true });
	}
</script>

<Heading level={1} text="Add a Client and ask to start" />
<p class="lede">
	Nobody matched your search, so this is a new record. Saving her also asks an Owner or Admin to
	approve the work.
</p>

{#if matches.length > 0}
	<MatchPrompt
		{matches}
		typed={client}
		onReuse={(existing) => {
			matches = [];
			onDone({
				reused: `${existing.given_name} ${existing.family_name}`,
				note: 'Her record was kept, what you typed applied as an edit, and one Request raised against it.',
				withRequest: true
			});
		}}
		onDifferentPerson={() => {
			matches = [];
			onDone({
				note: 'Duplicate created on a deliberate override, plus one Request.',
				withRequest: true
			});
		}}
	/>
{:else}
	<form onsubmit={submit}>
		<stack-l>
			{#if errors.length > 0}
				<Notice
					variant="error"
					message={`Still needed: ${errors.join('; ')}.`}
				/>
			{/if}

			<fieldset>
				<legend>Who she is</legend>
				<stack-l>
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
					<LabeledField label="What she goes by (optional)">
						{#snippet children({ id, describedBy, invalid })}
							<TextInput
								{id}
								{describedBy}
								{invalid}
								placeholder={client.given_name || 'Same as her first name'}
								value={client.preferred_name}
								onInput={(value) => onClient({ preferred_name: value })}
							/>
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
					<LabeledField
						label={demands === 'name-reach-dob' ? 'Date of birth' : 'Date of birth (optional)'}
					>
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
				</stack-l>
			</fieldset>

			<details bind:open={showAddress}>
				<summary>Her address — needed before a visit, not before a record</summary>
				<stack-l>
					{#each [['address_line1', 'Street address'], ['address_line2', 'Apartment, floor (optional)'], ['address_locality', 'Town or city'], ['address_region', 'State'], ['address_postal_code', 'ZIP']] as [key, label] (key)}
						<LabeledField {label}>
							{#snippet children({ id, describedBy, invalid })}
								<TextInput
									{id}
									{describedBy}
									{invalid}
									value={client[key as keyof ClientDraft]}
									onInput={(value) => onClient({ [key]: value })}
								/>
							{/snippet}
						</LabeledField>
					{/each}
				</stack-l>
			</details>

			<details bind:open={showPracticeFields}>
				<summary>What this Practice also asks</summary>
				<PracticeFieldsBlock values={custom} onChange={onCustom} />
			</details>

			<fieldset>
				<legend>The work you are asking to start</legend>
				<RequestBlock {request} onChange={onRequest} />
			</fieldset>

			<Button type="submit" label={`Save ${fullName(client)} and ask to start`} />
			<p class="hint">No Credit is spent until an Owner or Admin approves.</p>
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

	summary {
		cursor: pointer;
		font-weight: 600;
		margin-block-end: 0.5rem;
	}

	.hint {
		margin: 0;
		font-size: 0.8125rem;
		color: #555;
	}
</style>

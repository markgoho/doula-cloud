<script lang="ts">
	// PROTOTYPE (#372) -- throwaway. The Engagement Request's own facts: the kind
	// (#308's create-time control) and the due date (nullable, #353). No
	// Engagement exists yet, so ADR-0015's post-birth freeze on kind cannot bite
	// here -- it belongs to the Engagement's edit path, after approval.
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import type { Kind, RequestDraft } from './fixtures.js';

	interface Properties {
		request: RequestDraft;
		onChange: (patch: Partial<RequestDraft>) => void;
		showKindNote?: boolean;
	}

	let { request, onChange, showKindNote = true }: Properties = $props();
</script>

<stack-l>
	<RadioGroup
		legend="What kind of work is this?"
		options={[
			{ value: 'birth' as Kind, label: 'Birth — the pregnancy is ongoing' },
			{ value: 'postpartum' as Kind, label: 'Postpartum — support after a birth' }
		]}
		value={request.kind as Kind}
		onChange={(value) => onChange({ kind: value })}
	/>
	{#if showKindNote}
		<p class="hint">
			This is what the Practice sold. It can still be changed while the request is pending — withdraw
			it and ask again.
		</p>
	{/if}

	<LabeledField label="Expected due date">
		{#snippet children({ id, describedBy })}
			<input
				{id}
				type="date"
				aria-describedby="{describedBy ?? ''} {id}-hint"
				value={request.due_date}
				oninput={(event) => onChange({ due_date: event.currentTarget.value })}
			/>
			<p class="hint" id="{id}-hint">
				Optional. Leave it blank if there is no due date to give.
			</p>
		{/snippet}
	</LabeledField>

	<LabeledField label="Anything the approver should know">
		{#snippet children({ id, describedBy })}
			<textarea
				{id}
				aria-describedby={describedBy}
				rows="3"
				value={request.note}
				oninput={(event) => onChange({ note: event.currentTarget.value })}
			></textarea>
		{/snippet}
	</LabeledField>
</stack-l>

<style>
	.hint {
		margin: 0;
		font-size: 0.8125rem;
		color: #555;
		max-width: 60ch;
	}

	textarea {
		inline-size: 100%;
		font: inherit;
	}
</style>

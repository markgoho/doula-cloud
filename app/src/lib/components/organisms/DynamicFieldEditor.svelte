<script lang="ts">
	import { tick } from 'svelte';
	import { focusAfterMove, moveAnnouncement } from '#lib/fieldMoveFocus.js';
	import { FIELD_TYPES, isSelectType, type Field, type FieldType } from '#lib/planTemplate.js';
	import Button from '../atoms/Button.svelte';
	import Select from '../atoms/Select.svelte';
	import Textarea from '../atoms/Textarea.svelte';
	import TextInput from '../atoms/TextInput.svelte';
	import LabeledField from '../molecules/LabeledField.svelte';

	let {
		fields,
		onAdd,
		onRemove,
		onMoveUp,
		onMoveDown,
		onLabelChange,
		onTypeChange,
		onOptionsChange
	}: {
		fields: Field[];
		onAdd: (type: FieldType) => void;
		onRemove: (id: string) => void;
		onMoveUp: (id: string) => void;
		onMoveDown: (id: string) => void;
		onLabelChange: (id: string, label: string) => void;
		onTypeChange: (id: string, type: FieldType) => void;
		onOptionsChange: (id: string, options: string[]) => void;
	} = $props();

	let newFieldType = $state<FieldType>(FIELD_TYPES[0]);
	let list: HTMLUListElement;
	let announcement = $state('');

	// A moved row drops focus to the page (#1920); `focusAfterMove` says
	// where it goes back to, once the consumer has reordered `fields`.
	// The region is emptied first so that a text identical to the last
	// one (two "Untitled field" rows) is still a change, and is read
	// (#1955).
	async function move(id: string, direction: 'up' | 'down') {
		announcement = '';
		if (direction === 'up') {
			onMoveUp(id);
		} else {
			onMoveDown(id);
		}
		await tick();
		focusAfterMove(list, id, direction);
		announcement = moveAnnouncement(fields, id, rowName);
	}

	function optionsText(field: Field): string {
		return (field.options ?? []).join('\n');
	}

	// "Move up", "Move down" and "Remove" read as the same bare word on
	// every row (#515) -- a screen-reader user tabbing through, or scanning
	// a rotor's controls list, hears "Remove, Remove, Remove" with no row
	// context. This names the row a visually-hidden sibling joins each
	// Button to, the same fallback `field.label || 'Untitled field'` already
	// uses elsewhere for an unnamed field.
	function rowName(field: Field): string {
		return field.label || 'Untitled field';
	}

	function handleOptionsInput(id: string, value: string) {
		onOptionsChange(
			id,
			value.split('\n').map((line) => line.trim())
		);
	}
</script>

<ul bind:this={list}>
	{#each fields as field, index (field.id)}
		<li>
			<!--
				No wrapping label or LabeledField: this row's visible name is
				`field.label`'s own value, so a LabeledField would put the
				thing the field types beside the box that types it. ariaLabel
				is the recorded exception, the same one the Select right after
				it takes for the same reason (#492).
			-->
			<TextInput ariaLabel="Field label" value={field.label} onInput={(value) => onLabelChange(field.id, value)} />
			<Select
				ariaLabel="Field type"
				value={field.type}
				options={FIELD_TYPES}
				onChange={(value) => onTypeChange(field.id, value as FieldType)}
			/>
			{#if isSelectType(field.type)}
				<Textarea
					ariaLabel="Options, one per line"
					value={optionsText(field)}
					onInput={(next) => handleOptionsInput(field.id, next)}
					rows={2}
				/>
			{/if}
			<!-- v8 ignore start: Svelte-compiled attribute-diffing branches for the
			     dynamic id/describedBy strings below aren't reachable from
			     app-level interaction tests (no test renames a field mid-test),
			     only from Svelte's own reactivity internals -- the same
			     exception the select/option pair above takes. -->
			<Button
				label="Move up"
				describedBy="{field.id}-move-up-name"
				onClick={() => move(field.id, 'up')}
				ariaDisabled={index === 0}
			/>
			<span class="visually-hidden" id="{field.id}-move-up-name">{rowName(field)}</span>
			<Button
				label="Move down"
				describedBy="{field.id}-move-down-name"
				onClick={() => move(field.id, 'down')}
				ariaDisabled={index === fields.length - 1}
			/>
			<span class="visually-hidden" id="{field.id}-move-down-name">{rowName(field)}</span>
			<Button
				label="Remove"
				describedBy="{field.id}-remove-name"
				onClick={() => onRemove(field.id)}
			/>
			<span class="visually-hidden" id="{field.id}-remove-name">{rowName(field)}</span>
			<!-- v8 ignore stop -->
		</li>
	{/each}
</ul>
<!-- In the page from the first render, empty, so the first move is read (#1955). -->
<p class="visually-hidden" role="status">{announcement}</p>

<LabeledField label="New field type">
	{#snippet children({ id, describedBy, invalid })}
		<Select
			{id}
			{describedBy}
			{invalid}
			bind:value={() => newFieldType, (v) => (newFieldType = v as FieldType)}
			options={[...FIELD_TYPES]}
		/>
	{/snippet}
</LabeledField>
<Button label="Add field" onClick={() => onAdd(newFieldType)} />

<script lang="ts">
	import { FIELD_TYPES, isSelectType, type Field, type FieldType } from '#lib/planTemplate.js';
	import Button from '../atoms/Button.svelte';
	import Select from '../atoms/Select.svelte';
	import Textarea from '../atoms/Textarea.svelte';
	import TextInput from '../atoms/TextInput.svelte';

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

<ul>
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
			<!-- v8 ignore start: Svelte-compiled attribute-diffing branches for the
			     bound <select>/<option> pair below aren't reachable from app-level
			     interaction tests, only from Svelte's own reactivity internals -->
			<!-- eslint-disable-next-line svelte/no-restricted-html-elements -- this row has no wrapping <label>, so aria-label is its only accessible name; the Select atom has no aria-label/passthrough prop, so swapping it in would silently drop the field's name -->
			<select
				aria-label="Field type"
				value={field.type}
				onchange={(event_) => onTypeChange(field.id, event_.currentTarget.value as FieldType)}
			>
				{#each FIELD_TYPES as type (type)}
					<option value={type}>{type}</option>
				{/each}
			</select>
			<!-- v8 ignore stop -->
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
				onClick={() => onMoveUp(field.id)}
				disabled={index === 0}
			/>
			<span class="visually-hidden" id="{field.id}-move-up-name">{rowName(field)}</span>
			<Button
				label="Move down"
				describedBy="{field.id}-move-down-name"
				onClick={() => onMoveDown(field.id)}
				disabled={index === fields.length - 1}
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

<label>
	New field type
	<Select
		bind:value={() => newFieldType, (v) => (newFieldType = v as FieldType)}
		options={[...FIELD_TYPES]}
	/>
</label>
<Button label="Add field" onClick={() => onAdd(newFieldType)} />

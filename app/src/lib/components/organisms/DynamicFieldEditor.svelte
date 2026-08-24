<script lang="ts">
	import { FIELD_TYPES, isSelectType, type Field, type FieldType } from '#lib/planTemplate.js';
	import Button from '../atoms/Button.svelte';
	import Select from '../atoms/Select.svelte';

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
			<input
				type="text"
				aria-label="Field label"
				value={field.label}
				oninput={(event_) => onLabelChange(field.id, event_.currentTarget.value)}
			/>
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
				<textarea
					aria-label="Options, one per line"
					value={optionsText(field)}
					oninput={(event_) => handleOptionsInput(field.id, event_.currentTarget.value)}
				></textarea>
			{/if}
			<Button label="Move up" onClick={() => onMoveUp(field.id)} disabled={index === 0} />
			<Button
				label="Move down"
				onClick={() => onMoveDown(field.id)}
				disabled={index === fields.length - 1}
			/>
			<Button label="Remove" onClick={() => onRemove(field.id)} />
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

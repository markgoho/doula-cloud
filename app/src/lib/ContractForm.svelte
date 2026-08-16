<script lang="ts">
	import { mergeFieldLabel } from './contract.js';

	let {
		mergeFields,
		values,
		readOnly,
		onValueChange
	}: {
		mergeFields: string[];
		values: Record<string, string>;
		readOnly: boolean;
		onValueChange: (key: string, value: string) => void;
	} = $props();
</script>

<ul>
	{#each mergeFields as key (key)}
		<li>
			<label>
				<!-- v8 ignore start: Svelte's compiled null-guard on this text node is unreachable -- mergeFieldLabel always returns a string -->
				{mergeFieldLabel(key)}
				<!-- v8 ignore stop -->
				<input
					type="text"
					value={values[key] ?? ''}
					disabled={readOnly}
					oninput={(event_) => onValueChange(key, event_.currentTarget.value)}
				/>
			</label>
		</li>
	{/each}
</ul>

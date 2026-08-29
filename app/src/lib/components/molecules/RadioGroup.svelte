<script lang="ts" generics="T extends string">
	interface Option<T> {
		value: T;
		label: string;
	}

	interface Properties<T> {
		legend: string;
		name?: string;
		options: Option<T>[];
		value: T;
		onChange: (value: T) => void;
	}

	const generatedName = $props.id();

	let { legend, name = generatedName, options, value, onChange }: Properties<T> = $props();
</script>

<fieldset>
	<legend>{legend}</legend>
	<stack-l>
		<!-- v8 ignore start: only the compiled branch for "was this keyed
		     <cluster-l> added/removed from the DOM since the last render" is
		     unreachable here (Svelte's own each-block diffing internals, not
		     app code) -- the loop body itself is exercised by
		     "renders an option for each entry in options" in
		     RadioGroup.svelte.spec.ts -->
		{#each options as option (option.value)}
			<cluster-l>
				<input
					type="radio"
					id="{name}-{option.value}"
					{name}
					value={option.value}
					checked={option.value === value}
					onchange={() => onChange(option.value)}
				/>
				<label for="{name}-{option.value}">{option.label}</label>
			</cluster-l>
		{/each}
		<!-- v8 ignore stop -->
	</stack-l>
</fieldset>

<style>
	@layer components {
		fieldset {
			padding: 0;
			border: none;
		}

		legend {
			padding: 0;
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface);
		}

		input {
			accent-color: var(--color-primary);
			cursor: pointer;
		}

		input:focus-visible {
			outline: 2px solid var(--color-primary);
			outline-offset: 2px;
		}
	}
</style>

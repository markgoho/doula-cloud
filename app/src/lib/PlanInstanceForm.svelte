<script lang="ts">
	import type { Answers, Field } from './planInstance.js';

	let {
		fields,
		answers,
		onAnswerChange,
		onToggleOption
	}: {
		fields: Field[];
		answers: Answers;
		onAnswerChange: (fieldId: string, value: unknown) => void;
		onToggleOption: (fieldId: string, option: string) => void;
	} = $props();

	function textValue(field: Field): string {
		const value = answers[field.id];
		return typeof value === 'string' ? value : '';
	}

	function checkboxValue(field: Field): boolean {
		return answers[field.id] === true;
	}

	function selectedOptions(field: Field): string[] {
		const value = answers[field.id];
		return Array.isArray(value) ? (value as string[]) : [];
	}
</script>

<ul>
	{#each fields as field (field.id)}
		<li>
			{#if field.type === 'section_header'}
				<h3>{field.label}</h3>
			{:else if field.type === 'short_text'}
				<label>
					<!-- v8 ignore start: Svelte's compiled null-guard on this text node is unreachable -- field.label is always a string -->
					{field.label}
					<!-- v8 ignore stop -->
					<input
						type="text"
						value={textValue(field)}
						oninput={(e) => onAnswerChange(field.id, e.currentTarget.value)}
					/>
				</label>
			{:else if field.type === 'long_text'}
				<label>
					<!-- v8 ignore start: same unreachable null-guard as above -->
					{field.label}
					<!-- v8 ignore stop -->
					<textarea
						value={textValue(field)}
						oninput={(e) => onAnswerChange(field.id, e.currentTarget.value)}
					></textarea>
				</label>
			{:else if field.type === 'checkbox'}
				<label>
					<input
						type="checkbox"
						checked={checkboxValue(field)}
						onchange={(e) => onAnswerChange(field.id, e.currentTarget.checked)}
					/>
					<!-- v8 ignore start: same unreachable null-guard as above -->
					{field.label}
					<!-- v8 ignore stop -->
				</label>
			<!-- v8 ignore start: only the specific compiled branch for "was the
			     <select>/<option> pair added/removed from the DOM since the last
			     render" is actually unreachable here (Svelte's own reactivity
			     internals, not app code) -- but v8's line-range ignore can't target
			     that one branch without also covering the label/select/option
			     lines below it, which ARE exercised by "renders the selected
			     option for a single_select field" and "calls onAnswerChange when
			     the single_select changes" in PlanInstanceForm.svelte.spec.ts -->
			{:else if field.type === 'single_select'}
				<label>
					{field.label}
					<select
						value={textValue(field)}
						onchange={(e) => onAnswerChange(field.id, e.currentTarget.value)}
					>
						<option value="">--</option>
						{#each field.options ?? [] as option (option)}
							<option value={option}>{option}</option>
						{/each}
					</select>
				</label>
			{:else if field.type === 'multi_select'}
				<!-- v8 ignore stop -->
				<fieldset>
					<legend>
						<!-- v8 ignore start: same unreachable null-guard as above -->
						{field.label}
						<!-- v8 ignore stop -->
					</legend>
					{#each field.options ?? [] as option (option)}
						<label>
							<input
								type="checkbox"
								aria-label={`${field.label}: ${option}`}
								checked={selectedOptions(field).includes(option)}
								onchange={() => onToggleOption(field.id, option)}
							/>
							<!-- v8 ignore start: same unreachable null-guard as above -->
							{option}
							<!-- v8 ignore stop -->
						</label>
					{/each}
				</fieldset>
			{/if}
		</li>
	{/each}
</ul>

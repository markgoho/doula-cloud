<script lang="ts">
	import { isAnswerChecked, answerOptions, answerText, type Answers, type Field } from './planInstance.js';

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
		return answerText(answers, field.id);
	}

	function isCheckboxChecked(field: Field): boolean {
		return isAnswerChecked(answers, field.id);
	}

	function selectedOptions(field: Field): string[] {
		return answerOptions(answers, field.id);
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
						oninput={(event_) => onAnswerChange(field.id, event_.currentTarget.value)}
					/>
				</label>
			{:else if field.type === 'long_text'}
				<label>
					<!-- v8 ignore start: same unreachable null-guard as above -->
					{field.label}
					<!-- v8 ignore stop -->
					<textarea
						value={textValue(field)}
						oninput={(event_) => onAnswerChange(field.id, event_.currentTarget.value)}
					></textarea>
				</label>
			{:else if field.type === 'checkbox'}
				<label>
					<input
						type="checkbox"
						checked={isCheckboxChecked(field)}
						onchange={(event_) => onAnswerChange(field.id, event_.currentTarget.checked)}
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
					<!-- eslint-disable-next-line svelte/no-restricted-html-elements -- the empty option's value ("") differs from its visible label ("--"), which the Select atom cannot represent (it always uses one string for both); also, unlike Select's disabled placeholder option, this one stays selectable so the answer can be cleared -->
					<select
						value={textValue(field)}
						onchange={(event_) => onAnswerChange(field.id, event_.currentTarget.value)}
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

<script lang="ts">
	import { isAnswerChecked, answerOptions, answerText, type Answers, type Field } from '#lib/planInstance.js';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

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
				<LabeledField label={field.label}>
					{#snippet children({ id, describedBy, invalid })}
						<TextInput
							{id}
							{describedBy}
							{invalid}
							value={textValue(field)}
							onInput={(value) => onAnswerChange(field.id, value)}
						/>
					{/snippet}
				</LabeledField>
			{:else if field.type === 'long_text'}
				<label>
					<!-- v8 ignore start: same unreachable null-guard as above -->
					{field.label}
					<!-- v8 ignore stop -->
					<Textarea
						value={textValue(field)}
						onInput={(next) => onAnswerChange(field.id, next)}
					/>
				</label>
			{:else if field.type === 'checkbox'}
				<LabeledField label={field.label} orientation="inline">
					{#snippet children({ id, describedBy, invalid })}
						<Checkbox
							{id}
							{describedBy}
							{invalid}
							checked={isCheckboxChecked(field)}
							onChange={(checked) => onAnswerChange(field.id, checked)}
						/>
					{/snippet}
				</LabeledField>
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
					<!--
						A Practice writes its own option text, so this control's
						closed width is capped below at 100% of the space it is
						given (#549). What happens to an option still too long for
						that cap at 320px is a deliberate "clip, don't wrap": the
						closed box is UA-rendered, single-line chrome, so no
						`white-space`/`text-overflow` rule here would change it --
						Chromium already ellipsizes the displayed label, Safari and
						Firefox hard-clip it. Either way the full option text stays
						reachable to a Client: opening the native picker renders
						every <option> at its own natural width, unaffected by the
						closed control's capped width.
					-->
					<!-- eslint-disable-next-line svelte/no-restricted-html-elements -- the empty option's value ("") differs from its visible label ("--"), which the Select atom cannot represent (it always uses one string for both); also, unlike Select's disabled placeholder option, this one stays selectable so the answer can be cleared -->
					<select
						class="single-select"
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
						<!--
							No LabeledField: the option text is already visible as
							this checkbox's plain label below; ariaLabel only
							overrides the accessible name to prefix the question,
							since the same option text repeats across fields (#492).
						-->
						<label>
							<Checkbox
								ariaLabel={`${field.label}: ${option}`}
								checked={selectedOptions(field).includes(option)}
								onChange={() => onToggleOption(field.id, option)}
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

<style>
	@layer components {
		.single-select {
			max-width: 100%;
		}
	}
</style>

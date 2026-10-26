<script lang="ts">
	/*
	 * A Care Plan's or a Birth Plan's fields, answered by Staff on the
	 * Engagement page (ADR-0001).
	 *
	 * It draws no field itself (#2014). A Plan field and a Client field are
	 * the same five types a Practice defines, and `ClientFieldAnswers`
	 * already asks them to the bar -- a label above every control, a
	 * styled `Select`, a `CheckboxGroup` with one option per line. This
	 * form drew them by hand in a bulleted list, with a browser select and
	 * a boxed fieldset, and the two had drifted apart; ADR-0018's bar is
	 * that two consumers which should change together share one
	 * component. What is left here is the one thing a Plan has that a
	 * Client details section does not: `section_header` fields in among
	 * the questions. Each starts a group, its label a level-3 `Heading`
	 * under the page's Care Plan or Birth Plan heading, and the questions
	 * after it, up to the next header, are that group's.
	 */
	import { answerOptions, type Answers, type Field } from '#lib/planInstance.js';
	import type { FieldValue } from '#lib/intakeDraft.svelte.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import ClientFieldAnswers from '#lib/components/organisms/ClientFieldAnswers.svelte';

	interface Properties {
		fields: Field[];
		answers: Answers;
		onAnswerChange: (fieldId: string, value: unknown) => void;
		/*
		A multi-select's one ticked or unticked option.
		*/
		onToggleOption: (fieldId: string, option: string) => void;
	}

	let { fields, answers, onAnswerChange, onToggleOption }: Properties = $props();

	/*
	 * `ClientFieldAnswers` reports a multi-select as its whole list after
	 * the change; this form's callers take the one option that changed,
	 * which is the one value in either list and not the other -- one box
	 * changes per event.
	 */
	function handleChange(fieldId: string, value: FieldValue) {
		if (!Array.isArray(value)) {
			onAnswerChange(fieldId, value);
			return;
		}
		const before = answerOptions(answers, fieldId);
		const toggled = value.find((option) => !before.includes(option)) ?? before.find((option) => !value.includes(option));
		if (toggled !== undefined) onToggleOption(fieldId, toggled);
	}

	/* Care Plan and Birth Plan sit on one page, so each form's control
	   ids carry their own prefix. */
	const idPrefix = $props.id();

	interface Group {
		key: string;
		heading?: string;
		fields: Field[];
	}

	const groups = $derived.by(() => {
		let current: Group = { key: 'opening', fields: [] };
		const found: Group[] = [current];
		for (const field of fields) {
			if (field.type === 'section_header') {
				current = { key: field.id, heading: field.label, fields: [] };
				found.push(current);
			} else {
				current.fields.push(field);
			}
		}
		// Questions before the first header have no heading of their own;
		// with none, there is no group to draw.
		return found.filter((group) => group.heading !== undefined || group.fields.length > 0);
	});
</script>

<stack-l space="var(--space-8)">
	<!-- v8 ignore start: only the compiled branch for "was this keyed group
	     added/removed from the DOM since the last render" is unreachable
	     here (Svelte's own each-block diffing internals, not app code) --
	     the body is exercised by "starts a group at each section header" in
	     PlanInstanceForm.svelte.spec.ts -->
	{#each groups as group (group.key)}
		<stack-l space="var(--space-4)">
			{#if group.heading !== undefined}
				<Heading level={3} variant="card" text={group.heading} />
			{/if}
			<ClientFieldAnswers fields={group.fields} values={answers} onChange={handleChange} {idPrefix} />
		</stack-l>
	{/each}
	<!-- v8 ignore stop -->
</stack-l>

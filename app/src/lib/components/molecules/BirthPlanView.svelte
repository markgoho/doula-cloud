<script lang="ts">
	import { isAnswerChecked, answerOptions, answerText, type Answers, type Field } from '#lib/planInstance.js';

	let { fields, answers }: { fields: Field[]; answers: Answers } = $props();

	/**
	 * A section_header and the fields that follow it, up to the next one.
	 *
	 * The heading has to sit *outside* the `<dl>` rather than inside it:
	 * a description list may only directly contain `<dt>`, `<dd>` and
	 * `<div>`, so an `<h3>` between the pairs is invalid HTML and axe
	 * fails it (`definition-list`, WCAG 1.3.1). One list per section says
	 * what the old single list only implied -- these pairs belong under
	 * that heading -- and it is what a screen reader announces when it is
	 * asked how many items the list has.
	 */
	interface Section {
		key: string;
		heading: string | undefined;
		fields: Field[];
	}

	const sections = $derived.by(() => {
		// Fields before the first section_header have no heading of their
		// own; the leading section holds them, and renders as a bare list.
		const grouped: Section[] = [{ key: '', heading: undefined, fields: [] }];
		for (const field of fields) {
			if (field.type === 'section_header') {
				grouped.push({ key: field.id, heading: field.label, fields: [] });
			} else {
				grouped.at(-1)!.fields.push(field);
			}
		}
		return grouped.filter((section) => section.heading !== undefined || section.fields.length > 0);
	});

	function textValue(field: Field): string {
		const value = answerText(answers, field.id);
		return value === '' ? '—' : value;
	}

	function checkboxValue(field: Field): string {
		return isAnswerChecked(answers, field.id) ? 'Yes' : 'No';
	}

	function selectedOptions(field: Field): string {
		const options = answerOptions(answers, field.id);
		return options.length > 0 ? options.join(', ') : '—';
	}
</script>

{#each sections as section (section.key)}
	{#if section.heading !== undefined}
		<h3>{section.heading}</h3>
	{/if}
	{#if section.fields.length > 0}
		<dl>
			{#each section.fields as field (field.id)}
				<div>
					<dt>{field.label}</dt>
					{#if field.type === 'checkbox'}
						<dd>{checkboxValue(field)}</dd>
					{:else if field.type === 'multi_select'}
						<dd>{selectedOptions(field)}</dd>
					{:else}
						<dd>{textValue(field)}</dd>
					{/if}
				</div>
			{/each}
		</dl>
	{/if}
{/each}

<style>
	@layer components {
		/*
		 * A free-text answer can hold a URL, which a browser will not break
		 * on its own (#552). Scoped to the component that owns the answer
		 * markup so every caller gets it, not just this one.
		 */
		dd {
			overflow-wrap: anywhere;
		}
	}
</style>

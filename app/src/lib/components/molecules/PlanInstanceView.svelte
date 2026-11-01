<script lang="ts">
	import Heading from '#lib/components/atoms/Heading.svelte';
	import {
		isAnswerChecked,
		answerOptions,
		answerText,
		type Answers,
		type Field,
		type Masthead
	} from '#lib/planInstance.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import SummaryList, { type SummaryListRow } from '#lib/components/molecules/SummaryList.svelte';

	interface Properties {
		fields: Field[];
		answers: Answers;
		masthead?: Masthead;
		/**
		The level of each section_header's heading: one under the heading the view sits under.
		*/
		headingLevel: 2 | 3 | 4 | 5 | 6;
	}

	let { fields, answers, masthead, headingLevel }: Properties = $props();

	/*
	 * #2209, #2355: what names the plan wherever it is printed -- whose it
	 * is, her Doula while anyone is on the Engagement, whose Practice, and
	 * when it last changed -- before its first question, the same lines
	 * its PDF opens with. A reader handed the paper is not the person on
	 * the screen, so it is a plain label and value, read the same by both.
	 */
	function mastheadRows(named: Masthead): SummaryListRow[] {
		const doulaRows: SummaryListRow[] =
			named.doulas.length === 0
				? []
				: [{ label: named.doulas.length === 1 ? 'Doula' : 'Doulas', value: named.doulas.join(', ') }];
		return [
			{ label: 'Name', value: named.clientName },
			...doulaRows,
			{ label: 'Practice', value: named.practiceName },
			{ label: 'Last changed', value: formatCalendarDay(named.changedOn) }
		];
	}

	/**
	 * A section_header and the fields that follow it, up to the next one.
	 *
	 * The heading has to sit *outside* the `<dl>` rather than inside it:
	 * a description list may only directly contain `<dt>`, `<dd>` and
	 * `<div>`, so a heading between the pairs is invalid HTML and axe
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
		if (answers[field.id] === undefined) return '—';
		return isAnswerChecked(answers, field.id) ? 'Yes' : 'No';
	}

	function selectedOptions(field: Field): string {
		const options = answerOptions(answers, field.id);
		return options.length > 0 ? options.join(', ') : '—';
	}

	function answerValue(field: Field): string {
		if (field.type === 'checkbox') return checkboxValue(field);
		if (field.type === 'multi_select') return selectedOptions(field);
		return textValue(field);
	}

	/*
	 * Each section is a `SummaryList` (#2021), stacked: a question is a
	 * sentence a Practice wrote, and a sentence in a key column wraps to a
	 * ribbon beside its answer. Its value column also breaks a pasted URL
	 * (#552), which this component used to say for itself.
	 */
	function rowsFor(section: Section): SummaryListRow[] {
		return section.fields.map((field) => ({ label: field.label, value: answerValue(field) }));
	}
</script>

{#if masthead}
	<SummaryList rows={mastheadRows(masthead)} />
{/if}
{#each sections as section (section.key)}
	{#if section.heading !== undefined}
		<Heading level={headingLevel} text={section.heading} />
	{/if}
	{#if section.fields.length > 0}
		<SummaryList rows={rowsFor(section)} isStacked />
	{/if}
{/each}

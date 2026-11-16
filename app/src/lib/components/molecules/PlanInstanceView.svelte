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
	import { NOT_ANSWERED } from '#lib/intakeAnswers.js';
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
		/**
		Only the questions that have an answer, in the template's order, and a heading only over one that does. The record page reads a plan this way; the plan's own page and its PDF keep every question.
		*/
		answeredOnly?: boolean;
		/**
		The record page takes its column (`SummaryList`'s `width="column"`): its rows run to the column's edge, and where the column is wide enough for a 16rem key (53rem) each question sits beside its answer instead of above it.
		*/
		width?: 'measure' | 'column';
	}

	let { fields, answers, masthead, headingLevel, answeredOnly = false, width = 'measure' }: Properties = $props();

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
			} else if (!answeredOnly || answers[field.id] !== undefined) {
				grouped.at(-1)!.fields.push(field);
			}
		}
		return grouped.filter((section) =>
			answeredOnly ? section.fields.length > 0 : section.heading !== undefined || section.fields.length > 0
		);
	});

	function textValue(field: Field): string {
		const value = answerText(answers, field.id);
		return value === '' ? NOT_ANSWERED : value;
	}

	function checkboxValue(field: Field): string {
		if (answers[field.id] === undefined) return NOT_ANSWERED;
		return isAnswerChecked(answers, field.id) ? 'Yes' : 'No';
	}

	function selectedOptions(field: Field): string {
		const options = answerOptions(answers, field.id);
		return options.length > 0 ? options.join(', ') : NOT_ANSWERED;
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
	<SummaryList rows={mastheadRows(masthead)} {width} />
{/if}
{#each sections as section (section.key)}
	{#if section.heading !== undefined}
		<Heading level={headingLevel} text={section.heading} />
	{/if}
	{#if section.fields.length > 0}
		{#if width === 'column'}
			<div class="column-rows"><div class="column-size"><SummaryList rows={rowsFor(section)} isStacked {width} /></div></div>
		{:else}
			<SummaryList rows={rowsFor(section)} isStacked />
		{/if}
	{/if}
{/each}

<style>
	@layer components {
		/*
		 * A question is a sentence, so the rows stack it above its answer
		 * (#2021) until the row can give the question 16rem beside an answer
		 * of 20rem: then each sits beside its answer, and a plan of four
		 * answers loses four lines. Intrinsically, by flex wrapping, never by
		 * a query (ADR-0024).
		 */
		.column-size {
			font-size: var(--text-body-size);
		}

		/* tokens:ignore -- the question's and the answer's own widths, stated above */
		.column-rows :global(dl.stacked .key) {
			min-inline-size: 16rem;
		}

		/* tokens:ignore -- the question's and the answer's own widths, stated above */
		.column-rows :global(dl.stacked .value) {
			flex-basis: 20rem;
		}
	}
</style>

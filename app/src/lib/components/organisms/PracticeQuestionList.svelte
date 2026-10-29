<script module lang="ts">
	import type { FieldType } from '#lib/planTemplate.js';

	/** What a row of the list reads of a Plan Template field or a Client
	 * Field Template field, so each consumer passes its own `Field` through. */
	export interface ListedQuestion {
		id: string;
		type: FieldType;
		label: string;
	}
</script>

<script lang="ts">
	/*
	 * A Practice's own questions as GOV.UK's "Add another" list (#2193): a
	 * Summary list with one row per question, its words and its kind of
	 * answer, and a Change on each that opens that question's own page; then
	 * one "Add a question", which asks the kind of answer first. It replaced
	 * #2024's card per question with every control live on one screen, which
	 * showed sixteen decisions at once (MO-G12): here a row is one decision
	 * and the six kinds wait on the next page (the brief's Hick tie-break).
	 *
	 * The Plan Templates and the Client Fields screens both draw it, because
	 * they should change together (ADR-0018's 2026-10-25 bar). Who may change
	 * the list is the caller's to say: without `changeHref` a row has no
	 * Change, and without `addHref` there is no add, so a reader who may not
	 * change it reads the same list.
	 */
	import { FIELD_TYPE_NAMES } from '#lib/planTemplate.js';
	import Link from '../atoms/Link.svelte';
	import EmptyState from '../molecules/EmptyState.svelte';
	import SummaryList, { type SummaryListRow } from '../molecules/SummaryList.svelte';

	interface Properties {
		questions: readonly ListedQuestion[];
		/**
		The line an empty list shows.
		*/
		emptyMessage: string;
		/**
		Where a row's Change goes; absent for a reader who may not change it.
		*/
		changeHref?: (question: ListedQuestion) => string;
		/**
		Where "Add a question" goes; absent for a reader who may not add one.
		*/
		addHref?: string;
	}

	let { questions, emptyMessage, changeHref, addHref }: Properties = $props();

	type Row = SummaryListRow & { href?: string };

	const rows: Row[] = $derived(
		questions.map((question) => {
			const name = question.label || 'Untitled question';
			return {
				label: name,
				value: FIELD_TYPE_NAMES[question.type].name,
				changes: changeHref ? name : undefined,
				href: changeHref?.(question)
			};
		})
	);
</script>

<stack-l space="var(--space-group)">
	{#if questions.length === 0}
		<EmptyState message={emptyMessage} />
	{:else}
		<SummaryList {rows}>
			{#snippet action(row: Row, describedBy: string)}
				<Link href={row.href!} label="Change" variant="secondary" {describedBy} />
			{/snippet}
		</SummaryList>
	{/if}
	{#if addHref}
		<!-- The list's one act now that each question saves on its own page,
		     and the start of a short journey (kind, then the question), so
		     GOV.UK's start button: the accent spent once, on no screen that
		     also has a primary Button. -->
		<div>
			<Link href={addHref} label="Add a question" variant="start" />
		</div>
	{/if}
</stack-l>

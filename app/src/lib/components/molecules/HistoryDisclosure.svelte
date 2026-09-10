<script lang="ts" generics="T">
	/**
	 * A closed disclosure hanging off a row, holding that row's own
	 * history: opened on demand, fetched on the first open only, paged
	 * from there.
	 *
	 * #459 built this shape for the Staff roster's "Works from" value and
	 * deliberately left it inline -- "one consumer, and two identical
	 * consumers is the bar". #872 is the second consumer: the same roster
	 * row now also answers how the person came to hold her roles and her
	 * employment type. Two rows of the same table, two histories, one
	 * disclosure.
	 *
	 * A native `<details>`, so opening and closing costs no JavaScript and
	 * the keyboard and screen-reader behavior is the browser's own
	 * (GOV.UK's Details pattern, ADR-0021). The only script is the fetch
	 * the first open triggers, which the caller owns -- this component
	 * holds no history of its own, because the caller is the one that
	 * knows what a page of its history is and how to ask for the next.
	 */
	import type { Snippet } from 'svelte';
	import Button from '../atoms/Button.svelte';
	import Notice from '../atoms/Notice.svelte';
	import Text from '../atoms/Text.svelte';

	interface Properties {
		/**
		 * The disclosure's own words, e.g. "Work state history".
		 */
		label: string;
		/**
		 * Whose history this is. Every row of the roster carries the same
		 * disclosures, so the bare label names all of them alike and a
		 * screen reader's list of controls tells none of them apart (#667,
		 * the sibling of #515's Buttons). This name is appended to the
		 * summary, visually hidden, to tell them apart.
		 */
		subjectName: string;
		/**
		 * The entries loaded so far, or `undefined` while the first page is
		 * still in flight -- the difference between "nothing has come back
		 * yet" and "she has no history", which are two different sentences.
		 */
		items?: readonly T[];
		/**
		 * One entry's stable identity, for the keyed each block.
		 */
		key: (item: T) => string;
		/**
		 * One entry as the caller renders it, inside a list item.
		 */
		entry: Snippet<[T]>;
		/**
		 * What to say when the fetch failed. Empty means it did not.
		 */
		error?: string;
		/**
		 * What to say when there is no history at all.
		 */
		emptyMessage: string;
		/**
		 * Whether a further page exists behind the cursor.
		 */
		hasMore?: boolean;
		/**
		 * Whether the next page is in flight.
		 */
		isLoadingMore?: boolean;
		/**
		 * The words on the button that asks for the next page.
		 */
		loadMoreLabel: string;
		/**
		 * A prefix unique to this disclosure in the whole document, used
		 * for the id the "load more" button is described by. DataTable
		 * renders a row's snippet into both of its trees (#666), so a
		 * caller inside one must fold that view into this prefix or the
		 * describedby resolves to the copy that is not on screen.
		 */
		idPrefix: string;
		/**
		 * Called the first time the disclosure is opened.
		 */
		onOpen: () => void;
		/**
		 * Called when the reader asks for the next page.
		 */
		onLoadMore: () => void;
	}

	let {
		label,
		subjectName,
		items,
		key,
		entry,
		error = '',
		emptyMessage,
		hasMore = false,
		isLoadingMore = false,
		loadMoreLabel,
		idPrefix,
		onOpen,
		onLoadMore
	}: Properties = $props();

	/*
	 * The id the "load more" button is described by, named once and spent
	 * twice -- on the button's `describedBy` and on the hidden span it
	 * points at, which must agree or the description resolves to nothing.
	 *
	 * A plain expression rather than an interpolated attribute
	 * (`id="{idPrefix}-subject-name"`), which the compiler expands to
	 * `${idPrefix ?? ''}`: `idPrefix` is a required prop, so that `??`
	 * has a side no caller can ever take, and it would sit in the
	 * coverage report forever as a branch nothing reaches.
	 */
	const subjectNameId = $derived(`${idPrefix}-subject-name`);
</script>

<details ontoggle={(event) => event.currentTarget.open && onOpen()}>
	<!--
		A summary computes its accessible name from its own content, so
		GOV.UK's visually-hidden child applies literally here -- no id and
		no aria-describedby, which is also why this never meets #666's
		duplicate ids across DataTable's two trees. The space belongs to
		the summary's own text node rather than the span: accessible-name
		computation concatenates inline children without inserting one.
	-->
	<summary>{label} <span class="visually-hidden">for {subjectName}</span></summary>
	{#if error}
		<Notice variant="error" message={error} />
	{:else if !items}
		<Text text="Loading..." />
	{:else if items.length === 0}
		<Text text={emptyMessage} />
	{:else}
		<ol>
			{#each items as item (key(item))}
				<li>{@render entry(item)}</li>
			{/each}
		</ol>
		{#if hasMore}
			<Button
				label={loadMoreLabel}
				variant="secondary"
				size="sm"
				describedBy={subjectNameId}
				loading={isLoadingMore}
				onClick={onLoadMore}
			/>
			<span class="visually-hidden" id={subjectNameId}>{subjectName}</span>
		{/if}
	{/if}
</details>

<style>
	@layer components {
		/* A dated list of entries, not a bulleted aside: the order is the
		   history, so it is an <ol> with its markers off and the dates
		   doing the numbering's job. */
		ol {
			margin: 0;
			padding: 0;
			list-style: none;
			font-size: var(--text-body-sm-size);
			line-height: var(--text-body-sm-leading);
		}

		li {
			padding-block: var(--space-1);
		}

		/* WCAG 2.2 target size (minimum), which the axe archetype scan
		   enforces and caught on #459: the summary is a touch target, and
		   body-sm alone gives it a 21px line box. --space-6 is 24px
		   exactly, and the padding puts it clear of the boundary rather
		   than on it. */
		summary {
			font-size: var(--text-body-sm-size);
			line-height: var(--text-body-sm-leading);
			cursor: pointer;
			min-block-size: var(--space-6);
			padding-block: var(--space-1);
		}
	}
</style>

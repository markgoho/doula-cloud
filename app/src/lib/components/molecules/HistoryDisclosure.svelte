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
	 * Renders through `atoms/Details.svelte` (#1520), so opening and
	 * closing costs no JavaScript and the keyboard and screen-reader
	 * behavior is the browser's own (GOV.UK's Details pattern, ADR-0021).
	 * The only script is the fetch the first open triggers, which the
	 * caller owns -- this component holds no history of its own, because
	 * the caller is the one that knows what a page of its history is and
	 * how to ask for the next.
	 */
	import type { Snippet } from 'svelte';
	import Button from '../atoms/Button.svelte';
	import Details from '../atoms/Details.svelte';
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

<!--
	The summary's accessible name is its own text plus Details' hidden
	suffix (#667): a screen reader's list of controls tells this row's
	disclosure apart from every other row's, which never meets #666's
	duplicate ids across DataTable's two trees because nothing here is an
	id at all.
-->
<Details summary={label} hiddenSummary={`for ${subjectName}`} onToggle={(isOpen) => isOpen && onOpen()}>
	<!--
		The failure sits above whatever is already loaded rather than
		replacing it. A history pages, so the request that fails is
		usually the second one -- and answering "show me older changes"
		by taking away the changes she can already see loses the very
		thing she opened this for. GOV.UK's error-message pattern
		(ADR-0021) is a message beside the content, not instead of it.
	-->
	{#if error}
		<Notice variant="error" message={error} />
	{/if}
	{#if items && items.length > 0}
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
	{:else if items}
		<Text text={emptyMessage} />
	{:else if !error}
		<!--
			Nothing has come back yet -- but only when nothing went wrong
			either: a first page that failed says so above, and following it
			with "Loading..." would promise a request that is no longer in
			flight.
		-->
		<Text text="Loading..." />
	{/if}
</Details>

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
	}
</style>

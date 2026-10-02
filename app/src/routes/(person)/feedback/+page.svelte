<script lang="ts">
	/*
	 * The founder's list of Feedback (#1526, decided on #1499). One person
	 * reaches it: `staffauth.FounderOnly` answers everybody else 404, and
	 * this route's `load` turns that into the same "not found" page a URL
	 * with no route behind it gets.
	 *
	 * Two tables, and the order is the point. The first holds every piece
	 * whose GitHub issue did not open -- dead-lettered, or still retrying.
	 * A lapsed GITHUB_FEEDBACK_TOKEN dead-letters the outbox with nothing
	 * red anywhere else (#1500), so this is where it shows, above the list
	 * the founder came for. It keeps its heading when it is empty: "nothing
	 * is failing" is a fact he reads, not an absence he has to infer.
	 *
	 * No row carries the free text or a name. The list is for finding a
	 * piece; opening one is the act the BFF records.
	 *
	 * This page composes existing components and writes no CSS of its own:
	 * the frame is `ListPage`'s and each table is a `DataTable`, which
	 * already turns into one record per piece where a table does not fit
	 * (ADR-0024).
	 */
	import { untrack } from 'svelte';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		formatSentAt,
		issueLabel,
		kindLabel,
		loadFeedbackPage,
		type FeedbackSummary
	} from '#lib/founderFeedback.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	const failureMessage = 'Failed to load more feedback';

	// untrack: each list takes the load's first page once and grows from
	// there. Re-deriving from `data` would drop every page appended since.
	const unopened = new PaginatedList({
		first: untrack(() => data.unopened),
		loadPage: (cursor) => loadFeedbackPage(apiFetchWithSession, cursor, true),
		failureMessage
	});
	const all = new PaginatedList({
		first: untrack(() => data.all),
		loadPage: (cursor) => loadFeedbackPage(apiFetchWithSession, cursor),
		failureMessage
	});

	/*
	 * The time sent is the first column because it is the row's link, and
	 * the piece's own page is titled with the same instant.
	 */
	const columns = [
		{
			label: 'Sent',
			accessor: (piece: FeedbackSummary) => formatSentAt(piece.sentAt),
			datetimeAccessor: (piece: FeedbackSummary) => piece.sentAt
		},
		{ label: 'Kind', accessor: (piece: FeedbackSummary) => kindLabel(piece.kind) },
		{ label: 'Route', accessor: (piece: FeedbackSummary) => piece.routeId },
		{ label: 'Issue', accessor: (piece: FeedbackSummary) => issueLabel(piece.issue) }
	];

	// What the last attempt said: "401 Bad credentials" is the lapsed token.
	const unopenedColumns = [
		...columns,
		{ label: 'Last error', accessor: (piece: FeedbackSummary) => piece.issue.lastError ?? 'None recorded' }
	];

	function pieceHref(piece: FeedbackSummary): string {
		return resolve('/(person)/feedback/[feedbackId]', { feedbackId: piece.id });
	}
</script>

{#snippet intro()}
	<Text text="Every piece of feedback, newest first. Each time you open a piece, the read is recorded." tone="muted" />
{/snippet}

{#snippet content()}
	<!--
		data-sveltekit-preload-data="false" on both lists. app.html preloads a
		link's data on hover, and a row's link loads a piece -- the request
		the BFF records as a read. A hover is not a read, so no link here
		may load ahead of the click.
	-->
	<stack-l space="var(--space-4)" data-sveltekit-preload-data="false">
		<Heading level={2} variant="section" text="Issue not opened" />
		<DataTable
			columns={unopenedColumns}
			rows={unopened.items}
			rowHref={pieceHref}
			hasMore={unopened.hasMore}
			onLoadMore={() => unopened.loadMore()}
			isLoadingMore={unopened.isLoadingMore}
			emptyMessage="No attempt to open an issue has failed."
		/>
		{#if unopened.loadMoreError}
			<Notice message={unopened.loadMoreError} variant="error" />
		{/if}
	</stack-l>

	<stack-l space="var(--space-4)" data-sveltekit-preload-data="false">
		<Heading level={2} variant="section" text="All feedback" />
		<DataTable
			{columns}
			rows={all.items}
			rowHref={pieceHref}
			hasMore={all.hasMore}
			onLoadMore={() => all.loadMore()}
			isLoadingMore={all.isLoadingMore}
			emptyMessage="No feedback yet. A piece appears here as soon as somebody sends one."
		/>
		{#if all.loadMoreError}
			<Notice message={all.loadMoreError} variant="error" />
		{/if}
	</stack-l>
{/snippet}

<ListPage title="Feedback" {intro} {content} />

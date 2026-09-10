<script lang="ts">
	import HistoryDisclosure from '#lib/components/molecules/HistoryDisclosure.svelte';
	import { membershipChangeSentence } from '#lib/membershipHistory.js';
	import { formatActivityTimestamp } from '#lib/dates.js';
	import type { MembershipChange } from '#lib/staff.js';

	/*
	 * The longest realistic content, not a representative sample
	 * (ADR-0025, #537): a Membership event's sentence is the longest
	 * thing this component ever puts on one line, and the longest of
	 * those is a role change with all three roles on both sides. The
	 * actor is a real-length hyphenated name for the same reason -- "by
	 * <name>" rides on the end of that same line, so a polite name would
	 * measure a screen no Practice has.
	 */
	const entries: MembershipChange[] = [
		{
			eventId: 'event-3',
			action: 'roles_changed',
			actorName: 'Anne-Marie Ochieng-Whitfield',
			previousRoles: ['doula'],
			roles: ['owner', 'admin', 'doula'],
			createdAt: '2027-03-14T09:30:00Z'
		},
		{
			eventId: 'event-2',
			action: 'employment_type_changed',
			actorName: 'Anne-Marie Ochieng-Whitfield',
			previousEmploymentType: 'employee',
			employmentType: 'contractor',
			createdAt: '2026-11-01T12:00:00Z'
		},
		{
			eventId: 'event-1',
			action: 'joined',
			actorName: 'Anne-Marie Ochieng-Whitfield',
			roles: ['doula'],
			employmentType: 'employee',
			createdAt: '2026-08-28T12:00:00Z'
		}
	];

	// Every disclosure below is closed at rest, which is the state a
	// roster row actually shows. The continuum sweep opens each one
	// itself before it measures, so what is measured here is the revealed
	// content, not three words and a triangle.
</script>

<stack-l space="var(--space-6)">
	<h1>History disclosure</h1>

	<section>
		<h2>Loaded</h2>
		<HistoryDisclosure
			label="Membership history"
			subjectName="Anne-Marie Ochieng-Whitfield"
			items={entries}
			key={(change: MembershipChange) => change.eventId}
			emptyMessage="Nothing recorded."
			hasMore={true}
			loadMoreLabel="Show older membership changes"
			idPrefix="style-guide-loaded"
			onOpen={() => {}}
			onLoadMore={() => {}}
			entry={membershipEntry}
		/>
	</section>

	<section>
		<h2>Before the first page arrives</h2>
		<HistoryDisclosure
			label="Work state history"
			subjectName="Anne-Marie Ochieng-Whitfield"
			key={(change: MembershipChange) => change.eventId}
			emptyMessage="Nothing recorded."
			loadMoreLabel="Show older changes"
			idPrefix="style-guide-loading"
			onOpen={() => {}}
			onLoadMore={() => {}}
			entry={membershipEntry}
		/>
	</section>

	<section>
		<h2>Nothing recorded</h2>
		<HistoryDisclosure
			label="Membership history"
			subjectName="Anne-Marie Ochieng-Whitfield"
			items={[]}
			key={(change: MembershipChange) => change.eventId}
			emptyMessage="Nothing recorded."
			loadMoreLabel="Show older membership changes"
			idPrefix="style-guide-empty"
			onOpen={() => {}}
			onLoadMore={() => {}}
			entry={membershipEntry}
		/>
	</section>

	<section>
		<h2>The first page failed</h2>
		<HistoryDisclosure
			label="Membership history"
			subjectName="Anne-Marie Ochieng-Whitfield"
			key={(change: MembershipChange) => change.eventId}
			error="Failed to load membership history"
			emptyMessage="Nothing recorded."
			loadMoreLabel="Show older membership changes"
			idPrefix="style-guide-first-page-failed"
			onOpen={() => {}}
			onLoadMore={() => {}}
			entry={membershipEntry}
		/>
	</section>

	<section>
		<!--
			A later page failing keeps what is already on screen: answering
			"show me older changes" by taking away the changes she can
			already see loses the very thing she opened this for.
		-->
		<h2>A later page failed</h2>
		<HistoryDisclosure
			label="Membership history"
			subjectName="Anne-Marie Ochieng-Whitfield"
			items={entries}
			key={(change: MembershipChange) => change.eventId}
			error="Failed to load membership history"
			emptyMessage="Nothing recorded."
			loadMoreLabel="Show older membership changes"
			idPrefix="style-guide-error"
			onOpen={() => {}}
			onLoadMore={() => {}}
			entry={membershipEntry}
		/>
	</section>
</stack-l>

{#snippet membershipEntry(change: MembershipChange)}
	{membershipChangeSentence(change)} &mdash;
	<time datetime={change.createdAt}>{formatActivityTimestamp(change.createdAt)}</time>
	<span class="actor">by {change.actorName}</span>
{/snippet}

<style>
	@layer components {
		.actor {
			color: var(--color-on-surface-muted);
		}
	}
</style>

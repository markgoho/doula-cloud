<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { firstName, fullName } from '#lib/personName.js';
	import {
		loadClientDetail,
		pendingRequests,
		resolvedFieldGroups,
		resolvedFieldValueText,
		type ClientDetail,
		type EngagementRequestSummary,
		type EngagementSummary,
		type HistoryEntry,
		type ResolvedField
	} from '#lib/clientDetail.js';
	import { canAddDetails, fieldSectionIndexes } from '#lib/clientDetailsJourney.js';
	import { sectionSlug } from '#lib/intakeJourney.js';
	import type { SummaryListRow } from '#lib/components/molecules/SummaryList.svelte';
	import { kindLabel, withdrawRequest } from '#lib/engagementRequest.js';
	import { engagementStatusText } from '#lib/engagementDetail.js';
	import { eraseClient, loadEraseEligibility, type EraseEligibility, type UnsettledInvoice } from '#lib/clientErasure.js';
	import { formatAmount } from '#lib/invoice.js';
	import { isBlockedInvite, portalInviteStatusText } from '#lib/portalInvite.js';
	import { formatActivityTimestamp, formatCalendarDay, formatInstant } from '#lib/dates.js';
	import { formatAddress, formatPhoneNumber } from '#lib/contactDetails.js';
	import { takeOutcome } from '#lib/outcome.js';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import EmptyState from '#lib/components/molecules/EmptyState.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import SecondFactorNeeded from '#lib/components/molecules/SecondFactorNeeded.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import { detailsChangeHref } from './details/details.js';
	import type { PageProps as PageProperties } from './$types';

	/**
	A row of her record, and where its Change goes when it has one.
	*/
	interface ChangeRow extends SummaryListRow {
		href?: string;
	}

	// ADR-0017: a contractor Doula originates no Engagement Request, even
	// on a Client she is attached to (`engagementrequest.go`'s 403). `data`
	// stays optional -- this component's own spec renders it directly,
	// bypassing SvelteKit's load cycle, the same reason `clients/+page.svelte`
	// (#539) leaves it optional.
	let { data }: { data?: PageProperties['data'] } = $props();
	const isContractor = $derived(data?.isContractor ?? false);
	// #691: erasure is Owner-only. This is a courtesy -- EraseHandler and
	// EraseEligibilityHandler both enforce the real gate independently --
	// so the worst a wrong read here costs is the control staying hidden
	// from a genuine Owner for one page load, never a wider door than the
	// server actually opens.
	const isOwner = $derived(data?.isOwner ?? false);
	const hasSecondFactor = $derived(data?.hasSecondFactor ?? true);
	// The Practice's own name, off practices/[practiceId]/+layout.ts's
	// session (#835), names its questions (#2142), as the details
	// journey's first section page does (`intakeSections`). "the
	// Practice" stands in only where a spec renders the route bare.
	const practiceName = $derived(data?.session?.practiceName || 'the Practice');

	// #2142: back to the Clients list, as the Engagement page does
	// (#1998). One href for every role: a contractor Doula's Clients list
	// is the same route, serving her the Clients she is attached to.
	const clientsHref = $derived(resolve('/practices/[practiceId]/clients', { practiceId: page.params.practiceId! }));

	// What the save that led here did, if one did (#1710): her details
	// journey (a Change on a row, or its check page), a duplicate page, or a request to start work
	// that waits. Read once, while the page renders, so a reload says
	// nothing.
	const outcome = takeOutcome(page.url);

	let detail = $state<ClientDetail | undefined>();
	let error = $state('');
	// The signed-in Staff member's own id, read once alongside the Client
	// (#504): "Withdraw, available to the requester alone" is a fact about
	// who is looking, not about the Client record itself. Left blank on a
	// failed read rather than surfaced as a page error -- the endpoint
	// enforces the real rule (ADR-0017's requested_by = $1) regardless, so
	// the worst a lost session read costs is a Withdraw button that does
	// not render for its own requester this one time.
	let staffId = $state('');
	let withdrawingRequestId = $state('');
	let withdrawError = $state('');
	let withdrawnConfirmation = $state('');

	// #691: read once alongside the Client, Owner-only. Undefined while
	// loading (or on a read failure) is treated the same as "not
	// eligible yet" -- the erase control simply doesn't render rather
	// than risk offering a confirmation the endpoint would 409 on.
	let eligibility = $state<EraseEligibility | undefined>();
	let isEraseConfirmOpen = $state(false);
	let eraseError = $state('');

	// Safe with detail undefined (loading): read by the title, the
	// actions snippet and the sections array below, all of which are
	// constructed on every render regardless of which branch of
	// RecordDetail's template is actually showing. The heading names her
	// in full; every line under it, by her first name (the brief's Voice
	// rule 2, #2115).
	const title = $derived(detail ? fullName(detail) : '');
	const name = $derived(detail ? firstName(detail) : '');

	// The record's last section, and the one guard `eraseSection` relies on
	// (#1711): Owner-only, absent until the eligibility read answers, and
	// absent on a record already erased.
	const eraseSections = $derived(
		isOwner && eligibility && !detail?.erasedAt ? [{ heading: `Erase ${name}'s data`, content: eraseSection }] : []
	);

	/*
	 * Each row's Change (#2048): the details journey's door, naming the
	 * one question that row's value is asked by. The journey is the one
	 * way to change her record, so a row without a question -- the portal
	 * invite, a field the Practice no longer asks -- has no Change, and
	 * neither does an erased or merged record, whose edit `edit.go`
	 * refuses. Not gated on the contractor flag: edit follows read
	 * (ADR-0017), and `edit.go` refuses a Client she is not attached to.
	 */
	const canChange = $derived(detail !== undefined && canAddDetails(detail));
	const fieldSections = $derived(fieldSectionIndexes(detail?.resolvedFields ?? []));

	function changeHref(slug: string): string {
		return detailsChangeHref(page.params.practiceId!, page.params.clientId!, slug);
	}

	/**
	A row of her record, with a Change into the question that asks it.
	*/
	function changeable(label: string, value: string, slug: string, changes: string): ChangeRow {
		return canChange ? { label, value, changes, href: changeHref(slug) } : { label, value };
	}

	/**
	A row for one of the Practice's own questions: a Change into its
	section, or, for a question the Practice no longer asks (#2008), the
	BFF's note under the label and why there is no Change.
	*/
	function fieldRow(field: ResolvedField): ChangeRow {
		const value = resolvedFieldValueText(field) || '—';
		if (field.note) return { label: field.label, value, tag: field.note, status: 'Kept, not collected' };
		const section = fieldSections.get(field.fieldId);
		return section === undefined
			? { label: field.label, value }
			: changeable(field.label, value, sectionSlug(section), field.label);
	}

	// Under a section holding answers to questions the Practice stopped
	// asking (#2008): the drawing's sentence, to "you" (the brief's Voice).
	function noLongerAskedText(count: number): string {
		return count === 1
			? `Your Practice stopped asking this question. ${name}'s answer is kept and can be read. It cannot be changed, and it is not asked of a new Client.`
			: `Your Practice stopped asking these questions. ${name}'s answers are kept and can be read. The answers cannot be changed, and the questions are not asked of a new Client.`;
	}

	function newEngagementRequestHref(): string {
		return `/practices/${page.params.practiceId}/clients/${page.params.clientId}/engagement-requests/new`;
	}

	// Every value it is given is an instant (client/detail.go: an
	// Engagement's and an unsettled Invoice's createdAt, erasedAt,
	// stripeRedactionEligibleAt, a Request's requestedAt).
	function formattedDate(iso: string): string {
		return formatInstant(iso);
	}

	// A history row can be either shape; DataTable's Column is one
	// accessor per column, so each column below switches on the row it is
	// given rather than the two shapes getting two tables -- the AC is one
	// merged timeline, not two lists.
	function historyWho(entry: HistoryEntry): string {
		if (entry.type === 'client_event') {
			// Every actor kind arrives named (#1150). This screen used to
			// spell two of those names itself -- "DoulaCloud" for a system
			// actor, "Unknown staff" for a Staff member the join could no
			// longer reach -- which put ADR-0022's word for the product in a
			// second place and gave one Practice two words for one absence.
			return entry.clientEvent.actorName;
		}
		return entry.engagementRequest.requestedByName;
	}

	// #813 (ADR-0040). A merged Client's history is two trails, not one,
	// and it can never become one: each Client's entries are sealed under
	// her own key and the activity log is append-only, so the absorbed
	// woman's diffs may not be re-sealed under the survivor's. Every entry
	// says which record it was written against rather than letting the two
	// read as one continuous history that never happened.
	//
	// It rides in the What column rather than a fourth one: a fourth
	// column costs every row width at 320px to carry a fact almost no
	// Client's history has, and this way a screen reader hears the label
	// in the same cell as the thing it qualifies.
	//
	// It says "another record" rather than naming which. An absorbed row
	// is a tombstone and this screen deliberately never reads its name --
	// after an erasure there is no name left to read, and the label has to
	// mean the same thing before and after that.
	const MERGED_ENTRY_SUFFIX = ' (from another record, before the two were combined)';

	const mergedFrom = $derived(detail?.mergedFrom ?? []);

	// Says the act happened, when, and who did it -- the audit trail's own
	// question, answered from plaintext columns so it survives the
	// shredding of both keys.
	function mergeSummary(): string {
		const count = mergedFrom.length;
		const records = count === 1 ? 'another record' : `${count} other records`;
		const last = mergedFrom.at(-1)!;
		const who = last.mergedByName ?? 'a staff member';
		return `This Client was combined with ${records}. Most recently on ${formatActivityTimestamp(last.mergedAt)}, by ${who}. The histories below stay separate: each record's own entries are labeled, and combining them never joined the two.`;
	}

	function historyWhat(entry: HistoryEntry): string {
		return historyAction(entry) + (entry.fromMergedRecord ? MERGED_ENTRY_SUFFIX : '');
	}

	function historyAction(entry: HistoryEntry): string {
		if (entry.type === 'client_event') {
			switch (entry.clientEvent.eventType) {
				case 'created': {
					return 'Record created';
				}
				// ADR-0027. The one entry that outlives the rest: every
				// earlier entry's detail is unreadable after this, and
				// without a label of its own this would read as another
				// ordinary edit.
				case 'erased': {
					return 'Data erased on request';
				}
				default: {
					return 'Record updated';
				}
			}
		}
		const request = entry.engagementRequest;
		const label = kindLabel(request.kind);
		switch (request.state) {
			case 'pending': {
				return `${label} Engagement requested`;
			}
			case 'approved': {
				return `${label} Engagement approved`;
			}
			case 'refused': {
				return `${label} Engagement refused${request.reason ? `: ${request.reason}` : ''}`;
			}
			case 'withdrawn': {
				return `${label} Engagement request withdrawn`;
			}
			default: {
				return `${label} Engagement request`;
			}
		}
	}

	// ADR-0022: the shared ledger's own relative-or-absolute display
	// string, not this route's plain locale date -- the merged row union's
	// own `at` also feeds the datetimeAccessor below, carrying the exact
	// instant.
	function historyWhen(entry: HistoryEntry): string {
		return formatActivityTimestamp(entry.at);
	}

	function historyAt(entry: HistoryEntry): string {
		return entry.at;
	}

	// Best-effort, and deliberately not folded into the try/catch below:
	// a lost read here costs one Staff member her own Withdraw button for
	// one page load, not the Client record she came here to see.
	async function loadStaffId() {
		try {
			const response = await apiFetchWithSession('/api/staff/session');
			if (response.ok) {
				const session: { staffId: string } = await response.json();
				staffId = session.staffId;
			}
		} catch {
			// A network failure here must not become an unhandled rejection:
			// nothing awaits this call, and the cost of losing it is one
			// missing Withdraw button, not a broken page.
		}
	}

	// Withdraws request, then flips its own history entry to "withdrawn" in
	// place rather than reloading the whole Client -- pendingRequests()
	// re-derives from that same array, so the block disappears on the next
	// render for free. The disappearance itself is silent to a screen
	// reader, which is why a role=status confirmation follows it (#504 AC:
	// "keyboard-reachable and announced") rather than standing in as the
	// only signal that anything happened.
	async function handleWithdraw(request: EngagementRequestSummary) {
		withdrawError = '';
		withdrawnConfirmation = '';
		withdrawingRequestId = request.requestId;
		try {
			await withdrawRequest(apiFetchWithSession, page.params.practiceId!, request.requestId);
			const entry = detail!.history.find(
				(historyEntry) => historyEntry.type === 'engagement_request' && historyEntry.engagementRequest.requestId === request.requestId
			);
			if (entry?.type === 'engagement_request') {
				entry.engagementRequest.state = 'withdrawn';
			}
			withdrawnConfirmation = `${kindLabel(request.kind)} Engagement request withdrawn`;
		} catch (error_) {
			withdrawError = error_ instanceof Error ? error_.message : 'Failed to withdraw request';
		} finally {
			withdrawingRequestId = '';
		}
	}

	// Best-effort, the same reasoning as loadStaffId above: an Owner is
	// the only role that ever sees this read, and losing it costs one
	// missing erase control for one page load, not a broken page. Not
	// worth running for anyone else -- EraseEligibilityHandler is
	// Owner-only and would only 403.
	async function loadEligibility() {
		try {
			eligibility = await loadEraseEligibility(apiFetchWithSession, page.params.practiceId!, page.params.clientId!);
		} catch {
			// See above: the control simply doesn't render this time.
		}
	}

	// Names what #691's precheck already found, in the same words a
	// Practice would need to act on them -- amount, status, and when the
	// invoice was raised -- rather than the bare id EraseHandler's own
	// 409 names for a caller with no screen to render one.
	function unsettledInvoicesMessage(invoices: UnsettledInvoice[]): string {
		const list = invoices
			.map((invoice) => `${formatAmount(invoice.amountCents)} (${invoice.status}) from ${formattedDate(invoice.createdAt)}`)
			.join(', ');
		return `${name}'s data can't be erased yet. Settle or void ${list} first.`;
	}

	// ADR-0027's own content -- what is erased, what stays, Stripe's
	// 90-day hold, and the free text left as written -- as one short
	// sentence each, read aloud (#1744, the brief's Voice rules 4 and 6).
	// The Stripe sentence states Stripe's floor and names no date: the
	// eligibility date is per Stripe Customer (erase.go's stripeCustomer),
	// so one Client can have several, and the detail read shows the last
	// of them as stripeRedactionEligibleAt once the act has run. The free
	// text sentence names its three surfaces rather than "anything typed",
	// because erase.go does empty payment notes and reversal reasons.
	// Names {name} throughout rather than "her" -- #463's voice rule,
	// checked by copy.pronoun.usage.spec.ts.
	function eraseConsequence(): string {
		return `This permanently erases ${name}'s name, email, phone, address and date of birth. ${name}'s Engagements, Contracts, Invoices and Visits stay in your Practice's financial and clinical record. Stripe can't erase a payment record until 90 days after its invoice. Messages, signed Contracts and Plan Instance answers keep their words as written, even where they name ${name}. This cannot be undone.`;
	}

	// Re-fetches the Client rather than merging ErasureResponse's own
	// fields into `detail` -- ErasureResponse carries only erasedAt and
	// the Stripe date, not the redacted record, so a merge would leave
	// her name, email, phone, address and date of birth reading
	// pre-erasure until a manual reload, the exact fields this act just
	// destroyed. Rethrows on failure so ConfirmDialog's own contract
	// (left open on a rejected onConfirm) holds, and the 409 an
	// already-erased race or a newly-opened invoice still returns renders
	// through eraseError below rather than as raw response text.
	async function handleErase() {
		eraseError = '';
		try {
			await eraseClient(apiFetchWithSession, page.params.practiceId!, page.params.clientId!);
			detail = await loadClientDetail(apiFetchWithSession, page.params.practiceId!, page.params.clientId!);
		} catch (error_) {
			eraseError = error_ instanceof Error ? error_.message : 'Failed to erase this Client';
			throw error_;
		}
	}

	onMount(async () => {
		void loadStaffId();
		try {
			detail = await loadClientDetail(apiFetchWithSession, page.params.practiceId!, page.params.clientId!);
			// A tombstoned row is redirected, never rendered (ADR-0017's
			// amendment): every other field on this response is meaningless
			// placeholder data once mergedInto is set (detail.go), so this
			// hub must not draw from it at all.
			if (detail.mergedInto) {
				await goto(`/practices/${page.params.practiceId}/clients/${detail.mergedInto}`);
				return;
			}
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Client';
		}
		// Already-erased is unconditional and needs no precheck of its own
		// -- there is nothing left to settle, and the control never renders
		// once detail.erasedAt is set regardless of what this read answers.
		if (isOwner && !detail?.erasedAt) {
			void loadEligibility();
		}
	});
</script>

{#snippet actions()}
	<!--
		The header's one action (#2048). Her record is changed a row at a
		time, through each row's Change, so neither "Edit" nor a link to
		the whole details journey sits here any more.

		The hub is the only door to an Engagement Request (#496). The label
		names her rather than saying "her", so it reads on its own out of
		context and needs no describedBy sibling. The request screen itself
		then splits the wording by role -- "Ask to start work with" for an
		employee Doula, "Start work with" for an Owner or Admin -- because
		only there is the Credit cost known.
	-->
	{#if detail && !isContractor}
		<Link href={newEngagementRequestHref()} label="Start new work with {name}" />
	{/if}
{/snippet}

{#snippet summary()}
	<stack-l space="var(--space-4)">
		<!--
			An erased Client's record is a row of dashes and a placeholder
			name, which without this reads as a half-typed intake rather than
			an erasure (ADR-0027). The second notice is #394's "recorded
			somewhere a Practice can see it": while Stripe still holds
			transactions, the erasure is not finished, and saying so is the
			whole point of surfacing the date.
		-->
		{#if detail!.erasedAt}
			<Notice
				variant="info"
				message="This client's data was erased on request on {formattedDate(detail!.erasedAt)}."
			/>
			{#if detail!.stripeRedactionEligibleAt}
				<Notice
					variant="info"
					message="Payment records at Stripe are redactable from {formattedDate(
						detail!.stripeRedactionEligibleAt
					)} and have not been redacted yet."
				/>
			{/if}
		{/if}

		{#each pendingRequests(detail!.history) as request (request.requestId)}
			<!--
				cluster-l composes an existing layout primitive rather than
				writing new layout CSS (the temporary block on new components,
				CLAUDE.md) -- Notice carries no action slot of its own, so
				Withdraw sits beside it rather than inside it. Birth and
				postpartum can both be pending at once (ADR-0017's unique index
				is per kind), so the button names its own kind: two bare
				"Withdraw" controls in one tab order would be indistinguishable
				by ear.
			-->
			<cluster-l space="var(--space-3)" align="center">
				<Notice
					variant="info"
					message="{kindLabel(request.kind)} Engagement requested by {request.requestedByName} on {formattedDate(
						request.requestedAt
					)}"
				/>
				{#if request.requestedBy === staffId}
					<Button
						label="Withdraw {kindLabel(request.kind)} request"
						variant="secondary"
						size="sm"
						loading={withdrawingRequestId === request.requestId}
						onClick={() => handleWithdraw(request)}
					/>
				{/if}
			</cluster-l>
		{/each}

		{#if withdrawnConfirmation}
			<Notice variant="status" message={withdrawnConfirmation} />
		{/if}
		{#if withdrawError}
			<Notice variant="error" message={withdrawError} />
		{/if}
	</stack-l>
{/snippet}

<!-- GOV.UK's Summary list row action, as `CheckAnswers` draws it: one word,
     with what it changes in the visually-hidden text SummaryList joins to
     it, so the announced name is "Change, given name". -->
{#snippet changeAction(row: ChangeRow, describedBy: string)}
	<Link href={row.href!} label="Change" variant="secondary" {describedBy} />
{/snippet}

{#snippet whoSection()}
	<SummaryList
		rows={[
			changeable('Given name', detail!.givenName, 'name', 'given name'),
			changeable('Family name', detail!.familyName || '—', 'name', 'family name'),
			changeable('Preferred name', detail!.preferredName || '—', 'name', 'preferred name'),
			changeable(
				'Date of birth',
				detail!.dateOfBirth ? formatCalendarDay(detail!.dateOfBirth) : '—',
				'date-of-birth',
				'date of birth'
			)
		]}
		action={changeAction}
	/>
{/snippet}

{#snippet contactSection()}
	<SummaryList
		rows={[
			changeable('Email address', detail!.email || '—', 'email', 'email address'),
			changeable('Phone', formatPhoneNumber(detail!.phone) || '—', 'phone', 'phone number'),
			{ label: 'Portal invite', value: portalInviteStatusText(detail!) }
		] as ChangeRow[]}
		action={changeAction}
	>
		{#snippet value(row)}
			{row.value}
			<!--
				#785, moved here from the Clients list on #2022: the words alone
				name an act ("unblock the address") without saying where it
				happens, and Blocked email addresses is two clicks away under
				Settings. An anchor is the whole affordance, so the Rule of
				Least Power puts it here rather than in script.
			-->
			{#if row.label === 'Portal invite' && isBlockedInvite(detail!)}
				<Link
					href={resolve('/practices/[practiceId]/settings/blocked-addresses', {
						practiceId: page.params.practiceId!
					})}
					label="Blocked email addresses"
				/>
			{/if}
		{/snippet}
	</SummaryList>
{/snippet}

{#snippet addressSection()}
	<SummaryList
		rows={[
			changeable('Address', formatAddress(detail!) || '—', 'address', 'address')
		]}
		action={changeAction}
	/>
{/snippet}

<!--
	The Practice's own questions (ADR-0017's Practice-defined layer),
	named in the words a person would say (#2142): "What else Riverside
	Doula Collective asks", where the settings page says "The extra
	questions this Practice asks". One h2 over them all, each of the
	Practice's sections an h3 under it, rather than the old drawing's
	h2 per Practice section: `RecordDetail` keys its sections and their
	anchors on the heading, and a Practice can name two sections alike,
	or one "History", which would collide.
-->
{#snippet practiceQuestionsSection()}
	{@const fieldGroups = resolvedFieldGroups(detail!.resolvedFields)}
	{#if fieldGroups.length === 0}
		<!-- #2015: the Owner is the one who can add the questions, so only
		     the Owner is sent to the editor. -->
		<EmptyState
			message="{practiceName} has not added any questions of its own yet."
			action={isOwner
				? {
						label: 'Add a question',
						href: resolve('/practices/[practiceId]/settings/client-fields', {
							practiceId: page.params.practiceId!
						})
					}
				: undefined}
		/>
	{:else}
		<!--
			One SummaryList per section, not per field (#2051): a list per
			field gave each its own key column, so the values zig-zagged down
			the page. Keyed on index: a Practice can name two sections alike.
			Each row's Change opens the journey's page for its section (#2048);
			a field the Practice no longer asks has none.

			An answer to a question the Practice no longer asks is set apart
			from the current answers (#2008, the drawing "Sarah Beck - record,
			with an archived field"): last in its section, marked under its
			label with the BFF's own note, "Kept, not collected" where a
			Change would be, and one sentence under the list saying what that
			means.
		-->
		<stack-l space="var(--space-6)">
			{#each fieldGroups as group, groupIndex (groupIndex)}
				{@const archivedCount = group.fields.filter((field) => field.note).length}
				<stack-l space="var(--space-3)">
					{#if group.heading}
						<Heading level={3} variant="card" text={group.heading} />
					{/if}
					<SummaryList
						rows={[
							...group.fields.filter((field) => !field.note).map((field) => fieldRow(field)),
							...group.fields.filter((field) => field.note).map((field) => fieldRow(field))
						]}
						action={changeAction}
					/>
					{#if archivedCount > 0}
						<Text tone="variant" text={noLongerAskedText(archivedCount)} />
					{/if}
				</stack-l>
			{/each}
		</stack-l>
	{/if}
{/snippet}

{#snippet engagementsSection()}
	<!--
		#1511: each row's kind opens that Engagement's page, which holds
		its Contract, Care Plan, Birth Plan, Visits and Messages. A new
		Engagement has none of them, so no other screen links to it yet.
	-->
	<DataTable
		columns={[
			{ label: 'Kind', accessor: (row: EngagementSummary) => (row.kind === 'birth' ? 'Birth' : 'Postpartum') },
			{ label: 'Status', accessor: (row: EngagementSummary) => engagementStatusText(row.status) },
			{ label: 'Started', accessor: (row: EngagementSummary) => formattedDate(row.createdAt) }
		]}
		rows={detail!.engagements}
		rowHref={(row: EngagementSummary) =>
			resolve('/practices/[practiceId]/engagements/[engagementId]', {
				practiceId: page.params.practiceId!,
				engagementId: row.engagementId
			})}
		hasMore={false}
		emptyMessage="No Engagements yet."
	/>
{/snippet}

{#snippet historySection()}
	{#if mergedFrom.length > 0}
		<!--
			The plaintext explanation of why this history has two trails in
			it (#813). Every value here is a column on clients rather than
			a sealed diff, so it still reads after an erasure has shredded
			both Clients' keys -- which is exactly when it matters most,
			because by then every entry below says only that it cannot be
			read.

			Text, not Notice: every Notice variant carries a live-region
			role (alert or status), and this is standing explanation of what
			the table below is, not something that just happened. A second
			live region on the page would announce it over the withdraw
			confirmation that shares the screen.
		-->
		<Text step="body-sm" tone="muted" text={mergeSummary()} />
	{/if}
	<DataTable
		columns={[
			{ label: 'When', accessor: historyWhen, variant: 'meta' as const, datetimeAccessor: historyAt },
			{ label: 'What', accessor: historyWhat, variant: 'body' as const },
			{ label: 'Who', accessor: historyWho, variant: 'muted' as const }
		]}
		rows={detail!.history}
		hasMore={false}
		emptyMessage="No history yet."
	/>
{/snippet}

{#snippet eraseSection()}
	<!--
		#691: an unsettled invoice blocks the confirmation from ever
		opening, named the same way EraseHandler's own 409 would have -- so
		an Owner never reaches Erase only to see that 409 raw. Rendered only
		through `eraseSections`, which holds the guard on `eligibility`.
	-->
	{#if eligibility!.unsettledInvoices.length > 0}
		<Notice variant="info" message={unsettledInvoicesMessage(eligibility!.unsettledInvoices)} />
	{:else if !hasSecondFactor}
		<!-- #1532: erasing needs a second factor, so the control gives way to
		     what it needs and the way to set it up. -->
		<SecondFactorNeeded
			message="You need two-factor authentication before you can erase {name}'s data."
			returnTo={page.url.pathname}
		/>
	{:else}
		<stack-l space="var(--space-4)">
			<Text text="Erasing removes {name}'s name, contact details and date of birth for good. It cannot be undone." />
			<!-- The trigger's own label stays generic ("this Client's") rather
			     than repeating {name} -- ConfirmDialog's title and confirm
			     button both name her, and an identical accessible name on
			     both controls would make them indistinguishable by ear. -->
			<Button label="Erase this Client's data" variant="destructive" onClick={() => (isEraseConfirmOpen = true)} />
		</stack-l>
		<ConfirmDialog
			bind:open={isEraseConfirmOpen}
			title="Erase {name}'s data"
			consequence={eraseConsequence()}
			confirmLabel="Erase {name}'s data"
			error={eraseError}
			onConfirm={handleErase}
		/>
	{/if}
{/snippet}

<!--
	The erase control is the record's last section, not a line in the
	summary (#1711). In the summary it sat one row under the header's
	everyday links -- Edit, her details, Start new work -- and a
	destructive control that near the controls used every day is the wrong
	target to be near. GOV.UK keeps a warning button apart from the main
	actions; the account screen's "Delete your login" is the same shape,
	its own last block. Owner-only and absent until the eligibility read
	answers, the same courtesy gate the control always had; an erased
	record says so in the summary and offers nothing here.
-->
<RecordDetail
	{title}
	backHref={clientsHref}
	backLabel="Clients"
	{outcome}
	{summary}
	{actions}
	sections={[
		{ heading: `Who ${name} is`, content: whoSection },
		{ heading: `How to reach ${name}`, content: contactSection },
		{ heading: `Where ${name} lives`, content: addressSection },
		{ heading: `What else ${practiceName} asks`, content: practiceQuestionsSection },
		{ heading: 'Engagements', content: engagementsSection },
		{ heading: 'History', content: historySection },
		...eraseSections
	]}
	loading={detail || error ? undefined : 'Loading the Client'}
	loadError={error || undefined}
/>

<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		displayName,
		loadClientDetail,
		pendingRequests,
		resolvedFieldValueText,
		type ClientDetail,
		type EngagementRequestSummary,
		type EngagementSummary,
		type HistoryEntry
	} from '#lib/clientDetail.js';
	import { kindLabel, withdrawRequest } from '#lib/engagementRequest.js';
	import { eraseClient, loadEraseEligibility, type EraseEligibility, type UnsettledInvoice } from '#lib/clientErasure.js';
	import { formatAmount } from '#lib/invoice.js';
	import { formatActivityTimestamp } from '#lib/dates.js';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import type { PageProps as PageProperties } from './$types';

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
	// RecordDetail's template is actually showing.
	const name = $derived(detail ? displayName(detail) : '');

	function editHref(): string {
		return `/practices/${page.params.practiceId}/clients/${page.params.clientId}/edit`;
	}

	function newEngagementRequestHref(): string {
		return `/practices/${page.params.practiceId}/clients/${page.params.clientId}/engagement-requests/new`;
	}

	function formattedDate(iso: string): string {
		return new Date(iso).toLocaleDateString();
	}

	// A history row can be either shape; DataTable's Column is one
	// accessor per column, so each column below switches on the row it is
	// given rather than the two shapes getting two tables -- the AC is one
	// merged timeline, not two lists.
	function historyWho(entry: HistoryEntry): string {
		if (entry.type === 'client_event') {
			// ADR-0022: the third actor kind displays as "Doula Cloud", never
			// the engineering word "System" -- and only that kind lacks a
			// name, since Staff and Client actors are both joined in.
			return entry.clientEvent.actorKind === 'system' ? 'Doula Cloud' : (entry.clientEvent.actorName ?? 'Unknown staff');
		}
		return entry.engagementRequest.requestedByName;
	}

	function historyWhat(entry: HistoryEntry): string {
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
		return `${name}'s data can't be erased yet -- settle or void ${list} first.`;
	}

	// ADR-0027's own wording: what is destroyed, what survives, and the
	// two things that do not happen at once. The Stripe sentence
	// deliberately doesn't name one date -- ADR-0027's eligibility date is
	// per Stripe Customer, and a fresh Customer is made per invoice, so
	// two of her invoices months apart come due for redaction on two
	// different dates; naming a single "90 days from her newest invoice"
	// would be exactly the Client-wide date the ADR rejects. Names {name}
	// throughout rather than "her" -- #463's voice rule, checked by
	// copy.pronoun.usage.spec.ts.
	function eraseConsequence(): string {
		return `This permanently destroys ${name}'s identifying data: name, email, phone, address and date of birth. ${name}'s Engagements, Contracts, Invoices and Visits stay in the Practice's financial and clinical record. Stripe holds each Stripe payment record for 90 days from its own invoice before it can be redacted, so different invoices become eligible on different dates, and free text written by hand -- Messages, signed Contract wording, Plan Instance answers -- is never scrubbed. This cannot be undone.`;
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
		"Edit" alone doesn't say whose record it edits, the #513 violation
		CheckAnswers's Change links already solved -- a sibling
		visually-hidden span joined by aria-describedby, so the announced
		name becomes "Edit, Jane Smith" without a second visible word.
	-->
	<Link href={editHref()} label="Edit" describedBy="edit-client-name" />
	<span class="visually-hidden" id="edit-client-name">{name}</span>
	<!--
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
		{:else if isOwner && eligibility}
			<!--
				#691: an unsettled invoice blocks the confirmation from ever
				opening, named the same way EraseHandler's own 409 would have --
				so an Owner never reaches Erase only to see that 409 raw.
			-->
			{#if eligibility.unsettledInvoices.length > 0}
				<Notice variant="info" message={unsettledInvoicesMessage(eligibility.unsettledInvoices)} />
			{:else}
				<!-- The trigger's own label stays generic ("this Client's") rather
				     than repeating {name} -- ConfirmDialog's title and confirm
				     button both name her, and an identical accessible name on
				     both controls would make them indistinguishable by ear. -->
				<Button
					label="Erase this Client's data"
					variant="destructive"
					size="sm"
					onClick={() => (isEraseConfirmOpen = true)}
				/>
				<ConfirmDialog
					bind:open={isEraseConfirmOpen}
					title="Erase {name}'s data"
					consequence={eraseConsequence()}
					confirmLabel="Erase {name}'s data"
					onConfirm={handleErase}
				/>
			{/if}
			{#if eraseError}
				<Notice variant="error" message={eraseError} />
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

{#snippet whoSection()}
	<DescriptionList
		items={[
			{ label: 'Given name', value: detail!.givenName },
			{ label: 'Family name', value: detail!.familyName || '—' },
			{ label: 'Preferred name', value: detail!.preferredName || '—' },
			{ label: 'Date of birth', value: detail!.dateOfBirth || '—' }
		]}
	/>
{/snippet}

{#snippet contactSection()}
	<DescriptionList
		items={[
			{ label: 'Email', value: detail!.email || '—' },
			{ label: 'Phone', value: detail!.phone || '—' }
		]}
	/>
{/snippet}

{#snippet addressSection()}
	<DescriptionList
		items={[
			{
				label: 'Address',
				value:
					[
						detail!.addressLine1,
						detail!.addressLine2,
						detail!.addressLocality,
						detail!.addressRegion,
						detail!.addressPostalCode
					]
						.filter(Boolean)
						.join(', ') || '—'
			}
		]}
	/>
{/snippet}

{#snippet practiceDefinedFieldsSection()}
	{#if detail!.resolvedFields.length === 0}
		<Text text="No Practice-defined fields yet." />
	{:else}
		<stack-l space="var(--space-3)">
			{#each detail!.resolvedFields as field (field.fieldId)}
				{#if field.type === 'section_header'}
					<Heading level={3} variant="card" text={field.label} />
				{:else}
					<DescriptionList
						items={[
							{
								label: field.label,
								value: field.note ? `${resolvedFieldValueText(field) || '—'} (${field.note})` : resolvedFieldValueText(field) || '—'
							}
						]}
					/>
				{/if}
			{/each}
		</stack-l>
	{/if}
{/snippet}

{#snippet engagementsSection()}
	<DataTable
		columns={[
			{ label: 'Kind', accessor: (row: EngagementSummary) => (row.kind === 'birth' ? 'Birth' : 'Postpartum') },
			{ label: 'Status', accessor: (row: EngagementSummary) => row.status },
			{ label: 'Started', accessor: (row: EngagementSummary) => formattedDate(row.createdAt) }
		]}
		rows={detail!.engagements}
		hasMore={false}
		emptyMessage="No Engagements yet."
	/>
{/snippet}

{#snippet historySection()}
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

<RecordDetail
	title={name}
	{summary}
	{actions}
	sections={[
		{ heading: `Who ${name} is`, content: whoSection },
		{ heading: `How to reach ${name}`, content: contactSection },
		{ heading: `Where ${name} lives`, content: addressSection },
		{ heading: 'Practice-defined fields', content: practiceDefinedFieldsSection },
		{ heading: 'Engagements', content: engagementsSection },
		{ heading: 'History', content: historySection }
	]}
	loading={detail || error ? undefined : 'Loading the Client'}
	loadError={error || undefined}
/>

<script lang="ts">
	import Button from '#lib/components/atoms/Button.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import { formatActivityTimestamp } from '#lib/dates.js';
	import type { VoidRequestSummary } from '#lib/contract.js';

	/**
	 * Status display for a Contract on the Staff Engagement view: the raw
	 * status, a clear terminal-state indicator once Voided, the PDF
	 * download (#302) and the Void action. Void is offered only on a
	 * Signed Contract, since it is a one-way transition into the terminal
	 * Voided state. The download is not: it is offered whenever a signed
	 * PDF exists (hasSignedPdf, #1119), which a voided Contract's does --
	 * voiding cancels the agreement and deliberately keeps the evidence
	 * it was made (#299), and the endpoint has always served it. Two
	 * separate gates, for two separate reasons: hasSignedPdf is the
	 * existence gate, mirroring the endpoint's 404; the presence of
	 * onDownloadPdf is the permission gate, mirroring its 403 for a
	 * contractor. Gating the download on status instead was the drift
	 * #1119 fixed. The Client-portal
	 * Contract view stopped using this component on #212 (NH-G5): a Client
	 * reads `clientRegister.ts`'s own label and voided notice, never this
	 * component's Staff wording -- its own download control (#302) is
	 * built directly into that route instead.
	 *
	 * amountChangedAt (#968) surfaces here rather than only in the
	 * Engagement's activity ledger -- CLAUDE.md's "without hunting for
	 * it" AC -- the same `formatActivityTimestamp` relative-or-absolute
	 * rendering ADR-0022 gives every ledger entry, so a reader who has
	 * never opened the ledger still reads the same time format they
	 * would there.
	 *
	 * voidRequests/onRequestVoid/onDeclineVoidRequest (#971) are a Doula's
	 * other path to a void: she cannot call onVoid herself (#970 refuses
	 * her at the mount), so onRequestVoid is what the page wires up for
	 * her instead -- present exactly when onVoid is not, the same
	 * presence-means-permission convention onVoid/onDownloadPdf already
	 * use. onDeclineVoidRequest is the Owner/Admin side's other answer,
	 * wired up beside onVoid. voidRequests carries every request against
	 * the current Contract row regardless of who is looking, so a
	 * declined ask stays visible to whoever asked even after a different
	 * request on the same Contract has since been granted.
	 */
	let {
		status,
		hasSignedPdf = false,
		amountChangedAt,
		voidRequests = [],
		onVoid,
		onDownloadPdf,
		onRequestVoid,
		onDeclineVoidRequest
	}: {
		status: string;
		hasSignedPdf?: boolean;
		amountChangedAt?: string;
		voidRequests?: VoidRequestSummary[];
		onVoid?: () => Promise<void>;
		onDownloadPdf?: () => Promise<void>;
		onRequestVoid?: (reason: string) => Promise<void>;
		onDeclineVoidRequest?: (requestId: string, reason: string) => Promise<void>;
	} = $props();

	let isVoiding = $state(false);
	let isDownloading = $state(false);
	let error = $state('');

	// The open requests still waiting on a decision, and the ones already
	// declined -- a granted (voided) request needs no display of its own
	// here, since status itself already reads 'voided' by the time one
	// exists.
	const openVoidRequests = $derived(voidRequests.filter((request) => request.status === 'open'));
	const declinedVoidRequests = $derived(voidRequests.filter((request) => request.status === 'declined'));

	// "Request void" stays behind a control until asked for (the same
	// reveal BirthOutcomeSection uses for its own reason field), and
	// hides once a request is already open -- asking twice at once would
	// only ever 409, so there is nothing useful the control could do.
	let isRequestFormShown = $state(false);
	let requestReason = $state('');
	let requestError = $state('');
	let isRequestingVoid = $state(false);

	function openRequestForm() {
		isRequestFormShown = true;
	}

	function cancelRequestForm() {
		isRequestFormShown = false;
		requestReason = '';
		requestError = '';
	}

	async function handleRequestVoid() {
		requestError = '';
		if (!requestReason.trim()) {
			requestError = 'Enter why this contract needs to be voided';
			return;
		}
		isRequestingVoid = true;
		try {
			await onRequestVoid!(requestReason);
			isRequestFormShown = false;
			requestReason = '';
		} catch (error_) {
			requestError = error_ instanceof Error ? error_.message : 'Failed to request a void';
		} finally {
			isRequestingVoid = false;
		}
	}

	// Decline's own reveal-behind-a-control, one at a time -- opening a
	// second request's form closes whichever was open, since only one
	// decline reason is ever being typed at once.
	let declineFormRequestId = $state('');
	let declineReason = $state('');
	let declineError = $state('');
	let decliningRequestId = $state('');

	function openDeclineForm(requestId: string) {
		declineFormRequestId = requestId;
		declineReason = '';
		declineError = '';
	}

	function cancelDeclineForm() {
		declineFormRequestId = '';
		declineReason = '';
		declineError = '';
	}

	async function handleDeclineVoidRequest(requestId: string) {
		declineError = '';
		if (!declineReason.trim()) {
			declineError = 'Enter why this request is being declined';
			return;
		}
		decliningRequestId = requestId;
		try {
			await onDeclineVoidRequest!(requestId, declineReason);
			declineFormRequestId = '';
			declineReason = '';
		} catch (error_) {
			declineError = error_ instanceof Error ? error_.message : 'Failed to decline this request';
		} finally {
			decliningRequestId = '';
		}
	}

	// Only ever wired to the Void button below, which itself only renders
	// when onVoid is provided -- the non-null assertion reflects that,
	// rather than adding an unreachable defensive branch.
	async function handleVoid() {
		error = '';
		isVoiding = true;
		try {
			await onVoid!();
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to void contract';
		} finally {
			isVoiding = false;
		}
	}

	// Same shape as handleVoid above: the page's onDownloadPdf does the
	// fetch, Blob-to-object-URL conversion and the click that drives the
	// browser's own save, and throws on a failed fetch (#305 is the one
	// still live in local/CI) for this component to catch and show.
	async function handleDownloadPdf() {
		error = '';
		isDownloading = true;
		try {
			await onDownloadPdf!();
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to download signed Contract';
		} finally {
			isDownloading = false;
		}
	}
</script>

<p>Status: {status}</p>

{#if amountChangedAt}
	<p role="status">
		Price changed on <time datetime={amountChangedAt}>{formatActivityTimestamp(amountChangedAt)}</time>.
	</p>
{/if}

{#if status === 'voided'}
	<p role="status">Voided — this Contract is no longer active.</p>
{/if}

{#if hasSignedPdf && onDownloadPdf}
	<Button
		label="Download signed Contract (PDF)"
		icon="file-text"
		variant="secondary"
		onClick={handleDownloadPdf}
		loading={isDownloading}
	/>
{/if}

{#if status === 'signed' && onVoid}
	<Button label="Void Contract" onClick={handleVoid} loading={isVoiding} />
{/if}

<!--
	#971: a Doula's own path to a void, offered exactly where onVoid is
	not (she reaches this Contract but #970 refuses her the direct Void).
	Hidden once a request is already open -- a second ask right now would
	only ever 409 -- rather than shown and left to fail.
-->
{#if status === 'signed' && onRequestVoid}
	{#if openVoidRequests.length > 0}
		<p role="status">Void requested — waiting for an owner or admin to decide.</p>
	{:else if isRequestFormShown}
		<LabeledField label="Reason">
			{#snippet children({ id, describedBy, invalid })}
				<Textarea
					{id}
					{describedBy}
					{invalid}
					value={requestReason}
					onInput={(value) => (requestReason = value)}
				/>
			{/snippet}
		</LabeledField>
		{#if requestError}
			<p role="alert">{requestError}</p>
		{/if}
		<Button label="Send request" onClick={handleRequestVoid} loading={isRequestingVoid} />
		<Button label="Cancel" variant="secondary" onClick={cancelRequestForm} />
	{:else}
		<Button label="Request void" variant="secondary" onClick={openRequestForm} />
	{/if}
{/if}

<!--
	#971's other outcome: every void request already declined against
	this Contract, so the person who asked -- whose reason is right there
	beside it -- can tell a decline from a void without hunting the
	activity ledger for it, the same "without hunting" standard
	amountChangedAt's own notice above already meets.
-->
{#if status === 'signed'}
	<!-- v8 ignore start: Svelte-compiled attribute-diffing branches for
	     the two dynamic interpolations below aren't reachable from
	     app-level interaction tests (no test re-renders the same request
	     with a changed declineReason), only from Svelte's own reactivity
	     internals -- the same category OfferInbox.svelte's own comment
	     already names. -->
	{#each declinedVoidRequests as request (request.id)}
		<p role="status">Void request declined: {request.declineReason}</p>
	{/each}
	<!-- v8 ignore stop -->
{/if}

<!--
	The Owner/Admin side of a Doula's ask: each open request, who asked
	and why, and Decline behind its own reveal-behind-a-control (the
	Void button above already grants every open request at once).
-->
{#if status === 'signed' && onDeclineVoidRequest}
	{#each openVoidRequests as request (request.id)}
		<div>
			<!-- v8 ignore start: Svelte-compiled attribute-diffing branches
			     for the dynamic id/describedBy strings and the two
			     interpolations below aren't reachable from app-level
			     interaction tests (no test re-renders the same request with
			     changed fields, and no test decides between two open
			     requests at once), only from Svelte's own reactivity
			     internals -- the same category OfferInbox.svelte's own
			     comment already names. -->
			<p>{request.requestedByName} asked to void this contract: {request.reason}</p>
			{#if declineFormRequestId === request.id}
				<LabeledField label="Reason for declining">
					{#snippet children({ id, describedBy, invalid })}
						<Textarea
							{id}
							{describedBy}
							{invalid}
							value={declineReason}
							onInput={(value) => (declineReason = value)}
						/>
					{/snippet}
				</LabeledField>
				{#if declineError}
					<p role="alert">{declineError}</p>
				{/if}
				<Button
					label="Decline"
					describedBy="void-request-{request.id}-name"
					onClick={() => handleDeclineVoidRequest(request.id)}
					loading={decliningRequestId === request.id}
				/>
				<Button label="Cancel" variant="secondary" onClick={cancelDeclineForm} />
			{:else}
				<Button
					label="Decline"
					variant="secondary"
					describedBy="void-request-{request.id}-name"
					onClick={() => openDeclineForm(request.id)}
				/>
			{/if}
			<span class="visually-hidden" id="void-request-{request.id}-name"
				>{request.requestedByName}'s void request</span
			>
			<!-- v8 ignore stop -->
		</div>
	{/each}
{/if}

{#if error}
	<p role="alert">{error}</p>
{/if}

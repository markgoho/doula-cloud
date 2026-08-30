<script lang="ts">
	/*
	 * The Engagement Request screen (#496, ADR-0017): "Start new work with
	 * her", off the Client detail hub. The Doula states the kind and due
	 * date as part of the ask; the approver later approves or refuses
	 * exactly what was described and cannot amend it. Where the requester
	 * already holds approval authority (an Owner or an Admin), the request
	 * and its approval collapse into one act server-side
	 * (engagementrequest.RequestHandler) -- this screen only reads the
	 * response's state to know which happened.
	 *
	 * Two button labels, not one, read from the signed-in Staff member's
	 * own roles (the /session endpoint) -- the same UX-only mirror of the
	 * BFF's role gate the billing and website settings screens already use.
	 * The Credit cost and balance-after preview is Owner/Admin only,
	 * because reading the balance at all is (billing.GetBalanceHandler is
	 * ownerAndAdmin-gated, ADR-0008): a Doula's screen never attempts the
	 * call.
	 *
	 * AC4's "returns to the same, unlost form": Stripe's Checkout success
	 * and cancel URLs are hardcoded to the Billing page
	 * (billing/stripe_api_client.go), so there is no server-side way to
	 * carry a return-to URL through that round trip. sessionStorage carries
	 * the typed draft instead -- saved only at the moment an empty balance
	 * is discovered, restored on the next mount, and cleared on a
	 * successful submit, so a reader who never hits it never touches
	 * storage at all.
	 */
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadBalance } from '#lib/billing.js';
	import { displayName, loadClientDetail, type ClientDetail } from '#lib/clientDetail.js';
	import { requestEngagement, type NewEngagementRequest } from '#lib/engagementRequest.js';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import { SERVICE_PROBLEM, type FormError } from '#lib/formErrors.js';

	const KIND_NAME = 'engagement-request-kind';
	const kindFieldId = `${KIND_NAME}-birth`;
	const dueDateId = 'engagement-request-due-date';
	const noteId = 'engagement-request-note';

	let detail = $state<ClientDetail | undefined>();
	let roles = $state<string[]>([]);
	let balance = $state<number | undefined>();
	let loadError = $state('');

	let kind = $state<'' | 'birth' | 'postpartum'>('');
	let dueDate = $state('');
	let note = $state('');

	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);
	let hasNoCredits = $state(false);

	const isApprover = $derived(roles.includes('owner') || roles.includes('admin'));
	// ADR-0017's second-live-Engagement warning, read from the Client detail
	// already loaded rather than a separate call -- "warns, never refuses"
	// at request time so the requester can reconsider before submitting.
	const hasLiveEngagement = $derived(
		detail !== undefined && detail.engagements.some((engagement) => engagement.status !== 'completed')
	);
	// AC1's two button labels: an Owner or Admin reads as the purchase she
	// is making (ADR-0017's solo-Practice collapse), a Doula reads as the
	// ask she is sending.
	function submitLabelFor(client: ClientDetail): string {
		return isApprover ? `Start work with ${displayName(client)}` : `Ask to start work with ${displayName(client)}`;
	}
	const submitLabel = $derived(detail ? submitLabelFor(detail) : '');
	const hasIntroContent = $derived(hasLiveEngagement || (isApprover && balance !== undefined) || hasNoCredits);

	function detailHref(): string {
		return resolve('/practices/[practiceId]/clients/[clientId]', {
			practiceId: page.params.practiceId!,
			clientId: page.params.clientId!
		});
	}

	function billingHref(): string {
		return resolve('/practices/[practiceId]/billing', { practiceId: page.params.practiceId! });
	}

	function draftKey(): string {
		return `engagement-request-draft:${page.params.clientId}`;
	}

	interface Draft {
		kind: '' | 'birth' | 'postpartum';
		dueDate: string;
		note: string;
	}

	// Saved only at the moment a 402 is discovered, and read back once on
	// mount -- see the header comment. Wrapped in try/catch because
	// sessionStorage can throw in a private window with site data blocked,
	// and losing a draft is a far smaller failure than losing the screen.
	function saveDraft() {
		try {
			sessionStorage.setItem(draftKey(), JSON.stringify({ kind, dueDate, note } satisfies Draft));
		} catch {
			// Best effort: the round-trip back to Buy Credits still works,
			// she just retypes.
		}
	}

	function restoreDraft() {
		try {
			const saved = sessionStorage.getItem(draftKey());
			if (!saved) return;
			const parsed = JSON.parse(saved) as Draft;
			kind = parsed.kind;
			dueDate = parsed.dueDate;
			note = parsed.note;
		} catch {
			// A corrupted or unreadable draft is no worse than no draft.
		}
	}

	function clearDraft() {
		try {
			sessionStorage.removeItem(draftKey());
		} catch {
			// Nothing left to clean up if storage was never reachable.
		}
	}

	/*
	 * The due date is asked for on both kinds and demanded only on birth
	 * work. ADR-0017 makes `due_date` nullable "because a postpartum-only
	 * Engagement has none", and `parseRequestBody` accepts an empty
	 * dueDate, so demanding one on postpartum would refuse a request the
	 * endpoint and the schema both allow. It stays optional rather than
	 * hidden on postpartum: a postpartum package bought before the birth
	 * has a due date the approver wants to see.
	 */
	const isDueDateRequired = $derived(kind === 'birth');

	function findRefusals(): FormError[] {
		const found: FormError[] = [];
		if (kind === '')
			found.push({ message: 'Select whether this is birth or postpartum work', targetId: kindFieldId });
		if (isDueDateRequired && dueDate.trim() === '')
			found.push({ message: 'Enter the due date', targetId: dueDateId });
		return found;
	}

	function errorFor(targetId: string): string | undefined {
		return errors.find((entry) => entry.targetId === targetId)?.message;
	}

	onMount(async () => {
		restoreDraft();
		try {
			detail = await loadClientDetail(apiFetchWithSession, page.params.practiceId!, page.params.clientId!);
			const sessionResponse = await apiFetchWithSession(`/api/practices/${page.params.practiceId}/session`);
			if (sessionResponse.ok) {
				const body: { roles: string[] } = await sessionResponse.json();
				roles = body.roles;
			}
			if (roles.includes('owner') || roles.includes('admin')) {
				const balancePage = await loadBalance(apiFetchWithSession, page.params.practiceId!);
				balance = balancePage.balance;
			}
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load Client';
		}
	});

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errors = [];
		hasNoCredits = false;

		const refusals = findRefusals();
		if (refusals.length > 0) {
			errors = refusals;
			return;
		}

		// Safe: findRefusals above already refused an empty kind, so a
		// refusals.length === 0 reader always has one of the two values.
		const request: NewEngagementRequest = { kind: kind as 'birth' | 'postpartum', dueDate, note };

		isSubmitting = true;
		try {
			const result = await requestEngagement(
				apiFetchWithSession,
				page.params.practiceId!,
				page.params.clientId!,
				request
			);
			if (result.noCredits) {
				hasNoCredits = true;
				saveDraft();
				return;
			}
			clearDraft();
			await goto(detailHref());
		} catch (error_) {
			errors = [{ message: error_ instanceof Error && error_.message ? error_.message : SERVICE_PROBLEM }];
		} finally {
			isSubmitting = false;
		}
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={errors} />
{/snippet}

{#snippet formIntro()}
	<stack-l space="var(--space-4)">
		{#if hasLiveEngagement}
			<Notice
				variant="info"
				message="{detail ? displayName(detail) : 'This Client'} already has a live Engagement. A second one does not stop this request."
			/>
		{/if}

		{#if isApprover && balance !== undefined}
			<DescriptionList
				items={[
					{ label: 'Credit cost', value: '1 credit' },
					{ label: 'Balance after', value: String(balance - 1) }
				]}
			/>
		{/if}

		{#if hasNoCredits}
			<Notice variant="error" message="There are no credits left on this Practice's balance." />
			<Link href={billingHref()} label="Buy credits" />
		{/if}
	</stack-l>
{/snippet}

{#snippet requestFields()}
	<RadioGroup
		legend="Kind of work"
		name={KIND_NAME}
		options={[
			{ value: 'birth', label: 'Birth' },
			{ value: 'postpartum', label: 'Postpartum' }
		]}
		value={kind}
		onChange={(value: string) => (kind = value as 'birth' | 'postpartum')}
	/>
	<LabeledField
		id={dueDateId}
		label="Due date"
		hint={isDueDateRequired ? undefined : 'Optional for postpartum work'}
		error={errorFor(dueDateId)}
	>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				type="date"
				value={dueDate}
				onInput={(v) => (dueDate = v)}
				required={isDueDateRequired}
			/>
		{/snippet}
	</LabeledField>
	<LabeledField id={noteId} label="Note" hint="Anything the approver should know -- optional">
		{#snippet children({ id, describedBy, invalid })}
			<Textarea {id} {describedBy} {invalid} value={note} onInput={(v) => (note = v)} />
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet formActions()}
	<Button type="submit" label={submitLabel || 'Continue'} loading={isSubmitting} />
	<Link href={detailHref()} label="Cancel" variant="secondary" />
{/snippet}

<form onsubmit={handleSubmit} novalidate>
	<FormPage
		title={submitLabel || 'Start new work'}
		intro={hasIntroContent ? formIntro : undefined}
		fieldsets={detail ? [{ content: requestFields }] : []}
		errorSummary={errors.length > 0 ? errorSummary : undefined}
		actions={formActions}
		loading={detail || loadError ? undefined : 'Loading the Client'}
		loadError={loadError || undefined}
	/>
</form>

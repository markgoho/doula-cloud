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
	 * She also answers "Who is the Doula?" (#1596, ADR-0017's amendment on
	 * #1515): one employee Doula, or "No Doula yet". The list is the
	 * server's (engagementrequest.DoulasHandler) -- the roster for an Owner
	 * or an Admin, herself alone for anybody else -- and so is the one fact
	 * that selects an answer when the form opens: she is the only Doula at
	 * the Practice. In every other case nothing is selected and the submit
	 * is refused until she chooses. That pre-selection is a recorded
	 * departure from GOV.UK (docs/design/govuk-alignment.md). Radios and
	 * not a select, at any roster size: the departure rests on "No Doula
	 * yet" being on screen beside her own name and one press away, and a
	 * select would fold both behind a control.
	 *
	 * Two button labels, not one, read from the signed-in Staff member's
	 * own roles -- practices/[practiceId]/+layout.ts's already-resolved
	 * Membership (#835), the same UX-only mirror of the BFF's role gate
	 * the billing and website settings screens already use.
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
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { isOwnerOrAdmin } from '#lib/roles.js';
	import { loadBalance } from '#lib/billing.js';
	import { displayName, loadClientDetail, type ClientDetail } from '#lib/clientDetail.js';
	import type { PracticeSession } from '../../../../+layout.js';
	import {
		NO_DOULA_YET,
		doulaOptions,
		initialDoulaAnswer,
		loadRequestDoulas,
		requestEngagement,
		type NewEngagementRequest,
		type RequestDoulas
	} from '#lib/engagementRequest.js';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import { FormSubmission, orThrownErrors, type FormError } from '#lib/formSubmission.svelte.js';

	const KIND_NAME = 'engagement-request-kind';
	const kindFieldId = `${KIND_NAME}-birth`;
	const dueDateId = 'engagement-request-due-date';
	const DOULA_NAME = 'engagement-request-doula';
	const noteId = 'engagement-request-note';

	let detail = $state<ClientDetail | undefined>();
	let doulas = $state<RequestDoulas | undefined>();
	// Resolved once by practices/[practiceId]/+layout.ts (#835), not a
	// fetch of this page's own.
	const session = $derived((page.data as { session: PracticeSession }).session);
	let balance = $state<number | undefined>();
	let loadError = $state('');

	let kind = $state<'' | 'birth' | 'postpartum'>('');
	let dueDate = $state('');
	let note = $state('');
	// A staff id, NO_DOULA_YET, or '' while she has not answered.
	let doula = $state('');

	const submission = new FormSubmission();
	let hasNoCredits = $state(false);

	const isApprover = $derived(isOwnerOrAdmin(session));
	const doulaChoices = $derived(doulas ? doulaOptions(doulas.items, session.staffId) : []);
	// A refusal about the question points at its first option, which is
	// where GOV.UK sends an error link for a radio group. The first option
	// is the first person in the list, or "No Doula yet" at a Practice
	// with nobody to name.
	const doulaFieldId = $derived(radioFieldId(DOULA_NAME, doulaChoices[0]?.value ?? NO_DOULA_YET));
	// The BFF's own field names (engagementrequest.RequestBody's json
	// tags) mapped onto this form's controls, so a refusal it names lands
	// on the right one (#488).
	const requestFieldIds = $derived({ kind: kindFieldId, dueDate: dueDateId, doulaStaffId: doulaFieldId });
	// The empty-balance path is offered before the attempt as well as
	// after it (#1235): the balance is already loaded, so an approver on
	// an empty Practice is told she has nothing to spend rather than
	// reading "Balance after -1" or discovering it by pressing submit.
	// `balance <= 0` reads as "nothing to spend" because this request
	// always costs exactly one Credit (the hardcoded 'Credit cost: 1
	// credit' below) -- the same fixed cost the approval screen's own
	// `isBalanceEmpty` bakes into `balanceAfter < 0`. `isApprover &&` is
	// belt-and-braces: `balance` is only ever set for an approver below,
	// but the empty check should not depend on that living two functions
	// away to stay true.
	const isBalanceEmpty = $derived(isApprover && balance !== undefined && balance <= 0);
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
		// Optional: a draft saved before the question existed has none.
		doula?: string;
	}

	// Saved at the moment an empty balance is discovered -- either the
	// pre-submit $effect below, reading the balance already on screen, or
	// the 402 branch, for the race where it was not yet empty at load --
	// and read back once on mount, see the header comment. Wrapped in
	// try/catch because sessionStorage can throw in a private window with
	// site data blocked, and losing a draft is a far smaller failure than
	// losing the screen.
	function saveDraft() {
		try {
			sessionStorage.setItem(draftKey(), JSON.stringify({ kind, dueDate, note, doula } satisfies Draft));
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
			doula = parsed.doula ?? '';
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

	// Saved proactively whenever a zero balance is already on screen, not
	// only after a 402 (#1235) -- so a reader who types before or after
	// noticing the Buy Credits link doesn't lose it on that round trip.
	// Reading kind/dueDate/note/doula inside saveDraft() during this effect's own
	// run is what makes it re-save on every keystroke while true.
	$effect(() => {
		if (isBalanceEmpty) saveDraft();
	});

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
		if (doula === '')
			found.push({ message: 'Select who the Doula is, or select No Doula yet', targetId: doulaFieldId });
		return found;
	}

	onMount(async () => {
		restoreDraft();
		try {
			const [client, roster] = await Promise.all([
				loadClientDetail(apiFetchWithSession, page.params.practiceId!, page.params.clientId!),
				loadRequestDoulas(apiFetchWithSession, page.params.practiceId!)
			]);
			// A restored draft keeps its answer where the list still offers
			// it. Anything else -- no draft, or a draft naming a person who
			// is no longer in the list -- takes the opening answer: herself
			// where she is the only Doula at the Practice, and nothing
			// otherwise.
			const isOffered = doulaOptions(roster.items, session.staffId).some((option) => option.value === doula);
			if (!isOffered) doula = initialDoulaAnswer(roster, session.staffId);
			doulas = roster;
			detail = client;
			if (isApprover) {
				const balancePage = await loadBalance(apiFetchWithSession, page.params.practiceId!);
				balance = balancePage.balance;
			}
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load Client';
		}
	});

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		hasNoCredits = false;

		await submission.run(async () => {
			const refusals = findRefusals();
			if (refusals.length > 0) return refusals;

			// Safe: findRefusals above already refused an empty kind, so a
			// refusals.length === 0 reader always has one of the two values.
			// It refused an unanswered Doula question too, so `doula` is a
			// staff id or "No Doula yet", which travels as no doulaStaffId at all.
			const request: NewEngagementRequest = {
				kind: kind as 'birth' | 'postpartum',
				dueDate,
				note,
				doulaStaffId: doula === NO_DOULA_YET ? undefined : doula
			};

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
		}, orThrownErrors(requestFieldIds));
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet formIntro()}
	<stack-l space="var(--space-4)">
		{#if hasLiveEngagement}
			<Notice
				variant="info"
				message="{detail ? displayName(detail) : 'This Client'} already has a live Engagement. A second one does not stop this request."
			/>
		{/if}

		{#if hasNoCredits || isBalanceEmpty}
			<Notice variant="error" message="There are no credits left on this Practice's balance." />
			<Link href={billingHref()} label="Buy credits" />
		{:else if isApprover && balance !== undefined}
			<DescriptionList
				items={[
					{ label: 'Credit cost', value: '1 credit' },
					{ label: 'Balance after', value: String(balance - 1) }
				]}
			/>
		{/if}
	</stack-l>
{/snippet}

{#snippet requestFields()}
	<RadioGroup
		legend="Kind of work"
		name={KIND_NAME}
		error={submission.errorFor(kindFieldId)}
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
		error={submission.errorFor(dueDateId)}
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
	<RadioGroup
		legend="Who is the Doula?"
		name={DOULA_NAME}
		error={submission.errorFor(doulaFieldId)}
		options={doulaChoices}
		value={doula}
		onChange={(value: string) => (doula = value)}
	/>
	<LabeledField id={noteId} label="Note" hint="Anything the approver should know -- optional">
		{#snippet children({ id, describedBy, invalid })}
			<Textarea {id} {describedBy} {invalid} value={note} onInput={(v) => (note = v)} />
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet formActions()}
	<Button type="submit" label={submitLabel || 'Continue'} loading={submission.isSubmitting} />
	<Link href={detailHref()} label="Cancel" variant="secondary" />
{/snippet}

<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `FormPage` stacks each fieldset's content itself, so the `<form>` here owns the submit and arranges nothing; a `StackedForm` would put a second, empty stack around one child. -->
<form onsubmit={handleSubmit} novalidate>
	<FormPage
		title={submitLabel || 'Start new work'}
		intro={hasIntroContent ? formIntro : undefined}
		fieldsets={detail ? [{ content: requestFields }] : []}
		errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
		actions={formActions}
		loading={detail || loadError ? undefined : 'Loading the Client'}
		loadError={loadError || undefined}
	/>
</form>

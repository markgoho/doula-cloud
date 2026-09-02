<script lang="ts">
	/*
	 * Your account -- one screen, per person, not per Practice (#437).
	 *
	 * The decision this route exists to settle: a Staff member's work
	 * state is one fact about one person, however many Practices she
	 * works at. A contractor doula on three rosters does not work from
	 * New York at one of them and New Jersey at the other two. So a
	 * screen at /practices/[practiceId]/profile would be showing a global
	 * value inside a per-Practice frame, and a person who corrected it
	 * there would have every reason to believe she had corrected it only
	 * for that Practice. That is a small lie the layout would be telling
	 * on its own, before any copy got a chance to correct it. The route
	 * sits at the top level instead, where the value's reach and the
	 * screen's reach are the same shape.
	 *
	 * It follows from that that the way in cannot be the Staff roster:
	 * a Doula has no roster access at all, and she is exactly the person
	 * this screen is for. The link lives on the Staff layout header,
	 * beside sign-out, which every authenticated Staff screen carries.
	 *
	 * Data is read in onMount rather than a +page.ts load, matching every
	 * other authenticated Staff route in this app -- the whole app is a
	 * client-side SPA behind auth (`ssr = false` in src/routes/+layout.ts)
	 * and a load function would buy nothing but a second place to look.
	 */
	import { onMount } from 'svelte';
	import { apiFetchWithSession } from '#lib/api.js';
	import { refusalMessage, SERVICE_PROBLEM } from '#lib/formErrors.js';
	import { workStateCode, workStateName, workStateReportedOn } from '#lib/workStates.js';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import WorkStateField from '#lib/components/molecules/WorkStateField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import type { FormError } from '#lib/formErrors.js';
	import { loadAccountSession } from './session.svelte.js';

	const workStateId = 'account-work-state';

	let name = $state('');
	let reportedAt = $state('');
	// The full state name the <select> speaks; workStateCode() converts it
	// back to the USPS code the API stores on the way out.
	let selectedState = $state('');
	let isLoaded = $state(false);
	let loadError = $state('');
	let saveError = $state<FormError[]>([]);
	let savedState = $state('');
	let isSaving = $state(false);

	// #613: no verified-email flag is exposed here, so this is offered
	// unconditionally rather than only when unverified -- harmless either
	// way, since the outbox worker skips mailing an already-verified
	// account.
	let isResendingVerification = $state(false);
	let resendNotice = $state('');
	let resendError = $state('');

	async function handleResendVerification() {
		resendNotice = '';
		resendError = '';
		isResendingVerification = true;
		try {
			const response = await apiFetchWithSession('/api/staff/verify-email/request', { method: 'POST' });
			if (!response.ok) {
				resendError = await refusalMessage(response);
				return;
			}
			resendNotice = "We've sent a new verification link to your email address.";
		} catch {
			resendError = SERVICE_PROBLEM;
		} finally {
			isResendingVerification = false;
		}
	}

	async function loadAccount() {
		const result = await loadAccountSession();
		if (!result.ok) {
			// 404 means the verified identity has no staff row behind it --
			// signed in, but nobody here yet. Say so and render nothing to
			// edit, rather than offering a form whose save cannot land.
			loadError = result.message;
			return;
		}

		name = result.session.name;
		reportedAt = result.session.workStateReportedAt;
		selectedState = workStateName(result.session.workState);
		isLoaded = true;
	}

	onMount(loadAccount);

	/*
	 * One deliberate act: choose a state, press Save. No confirmation
	 * step, and that is a decision rather than an omission (#437).
	 *
	 * A confirmation dialog buys its friction with a promise that the act
	 * is hard to undo. This one is not: picking the previous state again
	 * puts it back, and both events are recorded either way, so the audit
	 * trail is richer for the round trip rather than damaged by it. And
	 * the failure this screen exists to fix is not a doula who changes
	 * her state carelessly -- it is a doula who moves and never says so,
	 * leaving her Practice's sales tax quietly wrong for years. Friction
	 * here pushes towards that failure, not away from it.
	 *
	 * If a confirmation is ever warranted, it is warranted on the
	 * consequence, which is why the consequence is stated above the field
	 * and before the choice instead.
	 */
	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		saveError = [];
		savedState = '';

		// The one question this page asks. Nothing else on it is editable.
		if (selectedState === '') {
			saveError = [{ message: 'Choose the state you work from', targetId: workStateId }];
			return;
		}

		isSaving = true;
		try {
			/*
			 * Sent every time, including when the state has not changed.
			 * Saying "yes, still New York, as of today" is a real thing to
			 * say: the reported date is the only staleness signal the design
			 * has, so a re-assertion moves it and is worth having. That is
			 * why the button is never disabled on an unchanged value and the
			 * request is never skipped -- an "optimization" here would
			 * silently delete the one thing this screen can tell an Owner
			 * reading the roster.
			 *
			 * There is no staffId in the path or the body. The endpoint
			 * only ever writes the caller's own row, which is how self-edit
			 * only is enforced where it can actually be enforced -- an Owner
			 * reads a work state on the roster and cannot write it (#415).
			 */
			const response = await apiFetchWithSession('/api/staff/work-state', {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ workState: workStateCode(selectedState) })
			});
			if (!response.ok) {
				saveError = [{ message: await refusalMessage(response) }];
				return;
			}

			const saved: { workState: string; workStateReportedAt: string } = await response.json();
			reportedAt = saved.workStateReportedAt;
			selectedState = workStateName(saved.workState);
			savedState = selectedState;
		} catch {
			// A throw here is the network, not her answer, so no field is
			// named.
			saveError = [{ message: SERVICE_PROBLEM }];
		} finally {
			isSaving = false;
		}
	}
</script>

{#snippet intro()}
	<!--
		The consequence, stated before the choice and not after it. A work
		state moves money: sales tax on a Practice's credits is apportioned
		over where its people work (TB-ST-128), so this field is the input
		to a bill somebody else pays. Someone changing it deserves to know
		that before she changes it, not in a confirmation dialog after.

		The second sentence closes off the question the first one opens.
		"Does correcting this claw back what I was charged last year?" No:
		#420 records the tax actually charged on each purchase row, so past
		receipts stand exactly as issued. A correction applies from today
		forward. Saying so here is cheaper than answering it in support.
	-->
	<Text
		text="Where you work sets how much sales tax your practice pays on the credits it buys. Changing it here changes that from today forward &mdash; purchases you have already made are not re-priced, and no receipt you have already been sent changes."
		tone="variant"
	/>
{/snippet}

{#snippet workState()}
	{#if reportedAt}
		<Text text={`Last confirmed ${workStateReportedOn(reportedAt)}.`} step="meta" tone="muted" />
	{/if}
	<WorkStateField
		id={workStateId}
		bind:value={selectedState}
		error={saveError.find((entry) => entry.targetId === workStateId)?.message}
	/>
	<!--
		Saving the same state again is a re-assertion, not a no-op -- see
		the comment on handleSubmit. Hence no `disabled` on an unchanged
		value.
	-->
{/snippet}

{#snippet errorSummary()}
	<ErrorSummary errors={saveError} />
{/snippet}

{#snippet actions()}
	<Button type="submit" label="Save work state" loading={isSaving} />
	<!--
		Confirmation sits where she just was -- immediately under the Save
		button she pressed, not in a banner at the top of a page she would
		have to scroll back up to read. Notice's status variant carries
		role="status", so a screen reader announces it politely wherever it
		is; a sighted reader is looking at the button. The "Last confirmed"
		line above the field moves to the new date at the same moment,
		which is the durable half of the same answer.

		Inside FormPage's actions region, the same placement `invite` (#425)
		uses -- FormPage owns the frame's width cap and gutters, and a
		sibling of the <form> below inherited neither (#474).
	-->
	{#if savedState}
		<Notice variant="status" message={`Saved. You work from ${savedState}.`} />
	{/if}
	<Button
		type="button"
		variant="secondary"
		label="Send a new verification link"
		loading={isResendingVerification}
		onClick={handleResendVerification}
	/>
	{#if resendNotice}
		<Notice variant="status" message={resendNotice} />
	{/if}
	{#if resendError}
		<Notice variant="error" message={resendError} />
	{/if}
{/snippet}

<!--
	A 404 `loadError` means the verified identity has no staff row behind
	it -- signed in, but nobody here yet -- so there is nothing to edit and
	offering a control whose save could never land would be worse than
	saying so. `loading`/`loadError` are `FormPage`'s own frame-reserving
	states (#480), which is why this is one call rather than a branch per
	state -- `novalidate`: this page refuses the submit, not the browser
	(#467).
-->
<form onsubmit={handleSubmit} novalidate>
	<FormPage
		title="Your account"
		{intro}
		fieldsets={isLoaded ? [{ legend: `Your details, ${name}`, content: workState }] : []}
		errorSummary={saveError.length > 0 ? errorSummary : undefined}
		{actions}
		loading={isLoaded || loadError ? undefined : 'Loading your account'}
		{loadError}
	/>
</form>

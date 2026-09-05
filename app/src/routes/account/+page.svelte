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
	import {
		getMultiFactorResolver,
		signInWithEmailAndPassword,
		signOut,
		TotpMultiFactorGenerator,
		type MultiFactorError,
		type MultiFactorResolver,
		type User
	} from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiBaseURL, apiFetchWithSession } from '#lib/api.js';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import {
		isMultiFactorAuthRequired,
		passwordReauthRefusal,
		refusalErrors,
		refusalMessage,
		SERVICE_PROBLEM,
		totpCodeRefusal
	} from '#lib/formErrors.js';
	import { workStateCode, workStateName, workStateReportedOn } from '#lib/workStates.js';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TotpCodeField from '#lib/components/molecules/TotpCodeField.svelte';
	import WorkStateField from '#lib/components/molecules/WorkStateField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import type { FormError } from '#lib/formErrors.js';
	import { loadAccountSession } from './session.svelte.js';

	const workStateId = 'account-work-state';

	let name = $state('');
	let email = $state('');
	let reportedAt = $state('');
	// The full state name the <select> speaks; workStateCode() converts it
	// back to the USPS code the API stores on the way out.
	let selectedState = $state('');
	// #606: whether *this session* showed a second factor at sign-in --
	// see SessionInfo's own doc comment for why that is not the same
	// question as "is one currently enrolled".
	let hasSecondFactor = $state(false);
	let isLoaded = $state(false);
	let loadError = $state('');
	let saveError = $state<FormError[]>([]);
	let savedState = $state('');
	let isSaving = $state(false);

	const mfaPasswordId = 'account-mfa-password';
	const mfaCodeId = 'account-mfa-code';

	// #606: voluntary removal of a second factor, from idle status through
	// the step-up reauth Identity Platform itself demands of an already
	// enrolled identity -- see the two-step shape below.
	let mfaStep = $state<'idle' | 'password' | 'code'>('idle');
	let mfaPassword = $state('');
	let mfaCode = $state('');
	let mfaErrors = $state<FormError[]>([]);
	let isMfaBusy = $state(false);

	/*
	 * The in-progress reauthentication Identity Platform is waiting on,
	 * kept across the password and code steps without being `$state` --
	 * the code step reads it once, to resolve, and the markup never does.
	 * Same shape as the login screen's own `mfaResolver`.
	 */
	let mfaResolver: MultiFactorResolver | undefined;

	function mfaErrorFor(targetId: string): string | undefined {
		return mfaErrors.find((entry) => entry.targetId === targetId)?.message;
	}

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
		email = result.session.email;
		reportedAt = result.session.workStateReportedAt;
		selectedState = workStateName(result.session.workState);
		hasSecondFactor = result.session.secondFactor;
		isLoaded = true;
	}

	onMount(loadAccount);

	function signOutOfMfaFirebaseSDK() {
		void signOut(getFirebaseAuth());
	}

	// #167's shared-device concern, the same one `/mfa/enroll` guards
	// against: a live client-side Identity Platform sign-in started by the
	// removal flow below must not survive her navigating away mid-flow.
	// Both handlers' own `signOut` calls already cover success and
	// failure; this is for the exit that skips them.
	onMount(() => signOutOfMfaFirebaseSDK);

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
				saveError = await refusalErrors(response, { workState: workStateId });
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

	function beginMfaRemoval() {
		mfaErrors = [];
		mfaPassword = '';
		mfaCode = '';
		mfaStep = 'password';
	}

	function cancelMfaRemoval() {
		mfaErrors = [];
		mfaPassword = '';
		mfaCode = '';
		mfaStep = 'idle';
	}

	/*
	 * The one thing both reauth steps below still have to do once Identity
	 * Platform accepts the credential: hand a fresh ID token to
	 * `DELETE /api/staff/mfa`, which reads it two ways at once --
	 * `authn.Begin` reads the `__session` cookie already on this request
	 * (`credentials: 'include'`), and `RequireRecentAuth` reads the Bearer
	 * token as a second, additional proof that the reauth just happened
	 * (api/internal/staffauth/reauth.go). Success ends every live session
	 * for the identity, this one included (`authn.EndAllSessions` keys on
	 * identity, not on the cookie that asked), so there is nothing left to
	 * show on this screen -- she is signed out and sent to log in again,
	 * the same shape `handleExpiredSession` (#lib/api.js) uses for any
	 * other session that has just ended out from under her.
	 *
	 * A failure here does not retry in place: the code step's resolver is
	 * single-use, so both callers fall back to the password step and ask
	 * her to start the step-up over.
	 */
	async function didRemoveSecondFactor(user: User): Promise<boolean> {
		const idToken = await user.getIdToken();
		const response = await fetch(`${apiBaseURL()}/api/staff/mfa`, {
			method: 'DELETE',
			credentials: 'include',
			headers: { Authorization: `Bearer ${idToken}` }
		});

		if (response.ok) {
			await signOut(getFirebaseAuth());
			await goto(`${resolve('/(signed-out)/login')}?sessionEnded=true`);
			return true;
		}

		await signOut(getFirebaseAuth());
		mfaErrors = [{ message: await refusalMessage(response) }];
		return false;
	}

	async function handleMfaPasswordSubmit() {
		mfaErrors = [];

		if (mfaPassword === '') {
			mfaErrors = [{ message: 'Enter your password', targetId: mfaPasswordId }];
			return;
		}

		isMfaBusy = true;
		try {
			const credential = await signInWithEmailAndPassword(getFirebaseAuth(), email, mfaPassword);
			const didRemove = await didRemoveSecondFactor(credential.user);
			if (!didRemove) mfaStep = 'password';
		} catch (error_) {
			if (isMultiFactorAuthRequired(error_)) {
				// Expected for an identity that already holds the factor
				// she is trying to remove: Identity Platform challenges the
				// second factor on every sign-in once one is enrolled.
				mfaResolver = getMultiFactorResolver(getFirebaseAuth(), error_ as MultiFactorError);
				mfaCode = '';
				mfaStep = 'code';
				return;
			}
			mfaErrors = [passwordReauthRefusal(error_, mfaPasswordId)];
		} finally {
			isMfaBusy = false;
		}
	}

	async function handleMfaCodeSubmit() {
		mfaErrors = [];

		if (mfaCode.trim() === '') {
			mfaErrors = [{ message: 'Enter the 6-digit code from your authenticator app', targetId: mfaCodeId }];
			return;
		}

		isMfaBusy = true;
		try {
			const assertion = TotpMultiFactorGenerator.assertionForSignIn(mfaResolver!.hints[0].uid, mfaCode);
			const credential = await mfaResolver!.resolveSignIn(assertion);
			const didRemove = await didRemoveSecondFactor(credential.user);
			if (!didRemove) mfaStep = 'password';
		} catch (error_) {
			mfaErrors = [totpCodeRefusal(error_, mfaCodeId)];
		} finally {
			isMfaBusy = false;
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

{#snippet mfaSection()}
	<!--
		#606: reads `hasSecondFactor` as the session-carried fact it is (see
		SessionInfo's own doc comment) rather than re-deriving it, which
		matches how staffauth.Middleware reads the same fact server-side.

		No nested `<form>` here -- the whole page is already one `<form>`
		(`handleSubmit`, below), and HTML forbids nesting one inside
		another. Every control in this fieldset is `type="button"` with its
		own `onClick`, the same shape "Send a new verification link" (in
		`actions`, below) already uses for a same-page secondary action.
	-->
	{#if hasSecondFactor}
		{#if mfaStep === 'idle'}
			<Text text="Turned on. You'll be asked for a code from your authenticator app when you sign in." />
			<Button type="button" variant="destructive" label="Remove" onClick={beginMfaRemoval} />
		{:else if mfaStep === 'password'}
			<Text text="Confirm your password to remove two-factor authentication." />
			<LabeledField id={mfaPasswordId} label="Password" error={mfaErrorFor(mfaPasswordId)}>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="password"
						value={mfaPassword}
						onInput={(value) => (mfaPassword = value)}
						required
						autocomplete="current-password"
					/>
				{/snippet}
			</LabeledField>
			<Button type="button" variant="destructive" label="Continue" loading={isMfaBusy} onClick={handleMfaPasswordSubmit} />
			<Button type="button" variant="secondary" label="Cancel" onClick={cancelMfaRemoval} disabled={isMfaBusy} />
		{:else}
			<TotpCodeField id={mfaCodeId} value={mfaCode} onInput={(value) => (mfaCode = value)} error={mfaErrorFor(mfaCodeId)} />
			<Button type="button" variant="destructive" label="Remove" loading={isMfaBusy} onClick={handleMfaCodeSubmit} />
			<Button type="button" variant="secondary" label="Cancel" onClick={cancelMfaRemoval} disabled={isMfaBusy} />
		{/if}
	{:else}
		<Text text="Not turned on." />
		<!--
			A link, not a button: this only ever navigates, the same reason
			`DataTable`'s rowHref cells and the practice-picker links are
			`Link` rather than `Button` + `goto`. `returnTo=/account` brings
			her back here once enrolment finishes (docs/design's link-text
			rule is why the label matches /mfa/enroll's own title verbatim).
		-->
		<Link href={`${resolve('/mfa/enroll')}?returnTo=${encodeURIComponent(resolve('/account'))}`} label="Set up two-factor authentication" />
	{/if}
	<!--
		Field-targeted refusals (a wrong password, a wrong code) already
		show beside their own control via LabeledField's `error` -- the same
		"Send a new verification link" precedent below, which shows its own
		failure as a plain Notice rather than a page-wide ErrorSummary. A
		service-level failure (the network, the DELETE call itself) names no
		field, so it is the one case shown here.
	-->
	{#if mfaErrors.length > 0 && !mfaErrors[0].targetId}
		<Notice variant="error" message={mfaErrors[0].message} />
	{/if}
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
		fieldsets={isLoaded
			? [
					{ legend: `Your details, ${name}`, content: workState },
					{ legend: 'Two-factor authentication', content: mfaSection }
				]
			: []}
		errorSummary={saveError.length > 0 ? errorSummary : undefined}
		{actions}
		loading={isLoaded || loadError ? undefined : 'Loading your account'}
		{loadError}
	/>
</form>

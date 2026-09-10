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
	import { signOut, type User } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiBaseURL, apiFetchWithSession } from '#lib/api.js';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { refusalErrors, refusalMessage, SERVICE_PROBLEM } from '#lib/formErrors.js';
	import { FormSubmission, orServiceProblem, orThrownMessage } from '#lib/formSubmission.svelte.js';
	import { rotateSavedCodes } from '#lib/mfaRecovery.js';
	import { triggerBlobDownload } from '#lib/blobDownload.js';
	import { workStateCode, workStateName, workStateReportedOn } from '#lib/workStates.js';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import ReauthPrompt from '#lib/components/molecules/ReauthPrompt.svelte';
	import WorkStateField from '#lib/components/molecules/WorkStateField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import { deleteOwnLogin } from '#lib/loginDeletion.js';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
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
	const saveSubmission = new FormSubmission();
	let savedState = $state('');

	// #606: voluntary removal of a second factor. The step-up Identity
	// Platform demands of an already-enrolled identity is `ReauthPrompt`'s
	// (#694), so all this screen holds is whether she has asked for it.
	let isRemovingSecondFactor = $state(false);
	const mfaSubmission = new FormSubmission();

	// #615: whether she is the only Owner of some Practice, which is the
	// whole population that ever holds saved recovery codes. Read off the
	// session rather than derived here -- the same session-carried fact
	// `hasSecondFactor` is, and the rotate endpoint re-derives it and
	// refuses on its own regardless (ADR-0006).
	let isSoleOwner = $state(false);
	const savedCodesSubmission = new FormSubmission();
	let savedCodes = $state<string[]>([]);
	let isConfirmingSavedCodes = $state(false);

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
		isSoleOwner = result.session.soleOwner;
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
		savedState = '';

		await saveSubmission.run(async () => {
			// The one question this page asks. Nothing else on it is editable.
			if (selectedState === '') {
				return [{ message: 'Choose the state you work from', targetId: workStateId }];
			}

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
				return await refusalErrors(response, { workState: workStateId });
			}

			const saved: { workState: string; workStateReportedAt: string } = await response.json();
			reportedAt = saved.workStateReportedAt;
			selectedState = workStateName(saved.workState);
			savedState = selectedState;
		}, orServiceProblem);
	}

	function beginMfaRemoval() {
		mfaSubmission.errors = [];
		isRemovingSecondFactor = true;
	}

	function cancelMfaRemoval() {
		mfaSubmission.errors = [];
		isRemovingSecondFactor = false;
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
	async function removeSecondFactor(user: User): Promise<void> {
		const idToken = await user.getIdToken();
		const response = await fetch(`${apiBaseURL()}/api/staff/mfa`, {
			method: 'DELETE',
			credentials: 'include',
			headers: { Authorization: `Bearer ${idToken}` }
		});

		// Thrown rather than returned: `ReauthPrompt` reads a rejection as
		// the act's own refusal and shows its words, which are the BFF's --
		// or the service sentence, where a 5xx had none of its own.
		if (!response.ok) {
			throw new Error(await refusalMessage(response));
		}

		await goto(`${resolve('/(signed-out)/login')}?sessionEnded=true`);
	}

	/*
	 * #615's saved recovery codes, and the one moment their plaintext ever
	 * exists outside her own notes.
	 *
	 * There is no "show me the ones I already have": the set minted when
	 * she became a Practice's sole Owner was discarded unread, and so is
	 * every replacement minted when she spends one. So being shown them
	 * and rotating them are one act, which is why the confirmation says
	 * plainly that anything she has already written down stops working.
	 */
	async function handleShowSavedCodes(): Promise<void> {
		await savedCodesSubmission.run(async () => {
			savedCodes = await rotateSavedCodes(apiFetchWithSession);
			isConfirmingSavedCodes = false;
		}, orThrownMessage);
	}

	/*
	 * One code per line, as a plain text file. The list stays on screen
	 * either way -- this is the half that works on a phone, where writing
	 * ten opaque strings down by hand is not a real option.
	 */
	function handleDownloadSavedCodes(): void {
		triggerBlobDownload(
			new Blob([savedCodes.join('\n')], { type: 'text/plain' }),
			'doula-cloud-recovery-codes.txt'
		);
	}

	/*
	 * #892: deleting her own login, the third direction ADR-0033 records
	 * -- a person asking Doula Cloud, where Erasure is a Client asking her
	 * Practice and Deletion is a Practice asking Doula Cloud.
	 *
	 * It lives here rather than on a Practice screen for the same reason
	 * the work state does: her login is one fact about one person, however
	 * many Practices she works at, and a control for it inside a
	 * per-Practice frame would be telling the same small lie about its own
	 * reach that this whole route exists to avoid.
	 *
	 * Unlike the work state above, this one does get a ConfirmDialog, and
	 * for the reason handleSubmit's comment says a confirmation has to
	 * earn: the act is irreversible. What it destroys and what it keeps is
	 * stated in the section itself, before the button, rather than only in
	 * the dialog -- someone deciding needs it while she decides, not once
	 * she has already pressed.
	 */
	let isDeleteLoginDialogOpen = $state(false);
	let isDeletingLogin = $state(false);
	let deleteLoginError = $state('');

	async function handleDeleteLogin() {
		deleteLoginError = '';
		isDeletingLogin = true;
		try {
			await deleteOwnLogin(apiFetchWithSession);
		} catch (error) {
			/*
			 * The last-Owner refusal names the Practices in the way, so it is
			 * rendered verbatim rather than replaced with a generic sentence.
			 *
			 * Rethrown so ConfirmDialog stays open and renders this inside
			 * itself (#804) -- she reads the named Practices right there,
			 * with the same Cancel and a confirm she can retry once she has
			 * acted on them, rather than losing the dialog's own context.
			 */
			deleteLoginError = error instanceof Error ? error.message : SERVICE_PROBLEM;
			throw error;
		} finally {
			isDeletingLogin = false;
		}

		/*
		 * Her session rows are already gone server-side and the cookie is
		 * cleared on the response, so nothing here is racing a live
		 * credential. Signing the client-side Identity Platform SDK out too
		 * is #167's shared-device rule, the same call the MFA flow above
		 * makes: the account is destroyed, but a stale SDK session object in
		 * this tab is not something to leave lying around.
		 */
		signOutOfMfaFirebaseSDK();
		await goto(resolve('/(signed-out)/login'));
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
		error={saveSubmission.errorFor(workStateId)}
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
		{#if isRemovingSecondFactor}
			<!--
				`insideForm`: this fieldset is inside the page's own `<form>`
				(below), and HTML forbids nesting one form inside another --
				see ReauthPrompt's own prop doc.
			-->
			<ReauthPrompt
				idPrefix="account-mfa"
				{email}
				prompt="Confirm your password to remove two-factor authentication."
				confirmLabel="Remove"
				confirmVariant="destructive"
				submission={mfaSubmission}
				onAuthenticated={removeSecondFactor}
				onCancel={cancelMfaRemoval}
				insideForm
			/>
		{:else}
			<Text text="Turned on. You'll be asked for a code from your authenticator app when you sign in." />
			<Button type="button" variant="destructive" label="Remove" onClick={beginMfaRemoval} />
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
		<Link href={`${resolve('/(signed-out)/mfa/enroll')}?returnTo=${encodeURIComponent(resolve('/account'))}`} label="Set up two-factor authentication" />
	{/if}
	<!--
		Field-targeted refusals (a wrong password, a wrong code) already
		show beside their own control via LabeledField's `error` -- the same
		"Send a new verification link" precedent below, which shows its own
		failure as a plain Notice rather than a page-wide ErrorSummary. A
		service-level failure (the network, the DELETE call itself) names no
		field, so it is the one case shown here.
	-->
	{#if mfaSubmission.errors.length > 0 && !mfaSubmission.errors[0].targetId}
		<Notice variant="error" message={mfaSubmission.errors[0].message} />
	{/if}
{/snippet}

{#snippet savedCodesSection()}
	<!--
		Offered only to a Practice's sole Owner, because she is the only
		person who holds these: everyone else has an Owner above her who can
		vouch for her, and offering a control here that would only ever 403
		would be implying she has a set she does not.

		Three states, never rendered together: the offer, the confirmation
		of what pressing it costs, and the codes themselves.
	-->
	{#if savedCodes.length > 0}
		<WarningText
			message="Write these down now. This is the only time they are shown, and each one works once."
		/>
		<!--
			Selectable plain text in a list, not inputs: they are read and
			copied, never edited, and an <ol> is what a numbered set of
			one-shot codes is. The same reasoning /mfa/enroll's own secret
			key uses.
		-->
		<ol class="codes">
			{#each savedCodes as code (code)}
				<li><code>{code}</code></li>
			{/each}
		</ol>
		<!--
			The way to keep them from a phone, where writing ten opaque
			strings on paper is not realistic. A text file lands in Files or
			Downloads on every mobile browser; the list above stays on
			screen for anyone who would rather copy them by hand.
		-->
		<Button
			type="button"
			variant="secondary"
			label="Download these codes"
			onClick={handleDownloadSavedCodes}
		/>
	{:else if isConfirmingSavedCodes}
		<WarningText
			message="Showing a new set replaces the one you have. Any code you wrote down earlier stops working."
		/>
		<Button
			type="button"
			label="Show a new set"
			loading={savedCodesSubmission.isSubmitting}
			onClick={handleShowSavedCodes}
		/>
		<Button
			type="button"
			variant="secondary"
			label="Cancel"
			disabled={savedCodesSubmission.isSubmitting}
			onClick={() => (isConfirmingSavedCodes = false)}
		/>
	{:else}
		<Text
			text="You are the only owner of a practice, so nobody else can vouch for you. Recovery codes are how you get back in if you lose your authenticator app."
		/>
		<Text
			text="They are shown once, when you ask for them. Doula Cloud keeps no copy it can read back to you."
			tone="variant"
		/>
		<Button
			type="button"
			label="Show my recovery codes"
			onClick={() => (isConfirmingSavedCodes = true)}
		/>
	{/if}
	<!--
		A `Notice` beside the control rather than the page-wide
		`ErrorSummary`, for the same recorded reason the two-factor section
		above gives: this fieldset's one button is not part of the form the
		summary is about (the work state), and a summary at the top of the
		page linking to nothing she can fix would send her away from the
		thing that failed. Nothing here can produce a field-targeted
		refusal -- there is no field -- so the guard is only that a refusal
		exists at all.
	-->
	{#if savedCodesSubmission.errors.length > 0}
		<Notice variant="error" message={savedCodesSubmission.errors[0].message} />
	{/if}
{/snippet}

{#snippet deleteLoginSection()}
	<!--
		Named in full, and split the way ADR-0033 splits it: what goes, and
		what stays. The second half matters as much as the first -- someone
		leaving a Practice has every reason to fear she is taking its record
		of her work with her, and she is not.

		Plain Text elements rather than a list component: three sentences
		read as prose here, and the alternative would be introducing a list
		this page has no other use for (docs/design's own rule about
		choosing a component being a layout decision).
	-->
	<Text text="Deleting your login ends your access to Doula Cloud everywhere, at once. It cannot be undone." />
	<Text
		text="This deletes your login and your membership of every practice you work at, and signs you out on every device."
		tone="variant"
	/>
	<Text
		text="Everything you did stays with the practices you did it for &mdash; the messages you sent, the contracts you are named on, the visits you worked. Doula Cloud keeps its own record that this account existed and that you deleted it."
		tone="variant"
	/>
	<Button
		type="button"
		variant="destructive"
		label="Delete your login"
		loading={isDeletingLogin}
		onClick={() => (isDeleteLoginDialogOpen = true)}
	/>
	<ConfirmDialog
		bind:open={isDeleteLoginDialogOpen}
		title="Delete your login"
		consequence="Your login, and your membership of every practice you work at, are deleted immediately. You cannot sign in again. Everything you did stays with those practices, and Doula Cloud keeps its own record that this account existed."
		confirmLabel="Delete your login"
		error={deleteLoginError}
		onConfirm={handleDeleteLogin}
	/>
{/snippet}

{#snippet errorSummary()}
	<ErrorSummary errors={saveSubmission.errors} />
{/snippet}

{#snippet actions()}
	<Button type="submit" label="Save work state" loading={saveSubmission.isSubmitting} />
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
					{ legend: 'Two-factor authentication', content: mfaSection },
					...(isSoleOwner ? [{ legend: 'Recovery codes', content: savedCodesSection }] : []),
					{ legend: 'Delete your login', content: deleteLoginSection }
				]
			: []}
		errorSummary={saveSubmission.errors.length > 0 ? errorSummary : undefined}
		{actions}
		loading={isLoaded || loadError ? undefined : 'Loading your account'}
		{loadError}
	/>
</form>

<style>
	@layer components {
		/* An ordered set of one-shot codes, with its markers kept: the
		   numbers are how she checks she has copied all ten. Wrapping is
		   intrinsic -- each code is one unbreakable word, so the list
		   reflows into the space it is given rather than at any width this
		   file names. */
		.codes {
			margin: 0;
			padding-inline-start: var(--space-6);
		}

		.codes li {
			padding-block: var(--space-1);
		}

		.codes code {
			font-size: var(--text-body-size);
			/* Long enough to reach past a 320px column on its own, so it is
			   allowed to break rather than pushing the page sideways. */
			overflow-wrap: anywhere;
		}
	}
</style>

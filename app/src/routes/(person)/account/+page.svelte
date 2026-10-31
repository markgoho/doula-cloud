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
	 *
	 * ## A summary of facts, not one long form (#2044)
	 *
	 * The page is the facts about her, in one `SummaryList`, and each row
	 * says how it changes: the name and the work state each open a question
	 * page of their own that saves and comes back here with a status
	 * Notice; the second factor is set up on /mfa/enroll or removed in
	 * place behind a password; the email takes a resend. Deleting the
	 * login is last and apart. It was five unrelated `FormPage` fieldsets,
	 * with two primary buttons and the resend landing under the delete
	 * section (docs/design/govuk-alignment.md, "Sections of a record page").
	 */
	import { onMount } from 'svelte';
	import { signOut, type User } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiBaseURL, apiFetchWithSession } from '#lib/api.js';
	import { staffLoginAfterSessionEnded } from '#lib/sessionEnded.js';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { refusalMessage, SERVICE_PROBLEM } from '#lib/formErrors.js';
	import { FormSubmission, orThrownMessage } from '#lib/formSubmission.svelte.js';
	import type { Membership } from '#lib/landing.js';
	import { rotateSavedCodes } from '#lib/mfaRecovery.js';
	import { takeOutcome } from '#lib/outcome.js';
	import { triggerBlobDownload } from '#lib/blobDownload.js';
	import { workStateName, workStateReportedOn } from '#lib/workStates.js';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import ReauthPrompt from '#lib/components/molecules/ReauthPrompt.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import { deleteOwnLogin } from '#lib/loginDeletion.js';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import { loadAccountSession } from '../session.svelte.js';

	// Read while the page renders, before `goto` settles (#1710): what the
	// name or work-state question page saved, said once.
	const outcome = takeOutcome(page.url);

	let firstName = $state('');
	let lastName = $state('');
	let email = $state('');
	let reportedAt = $state('');
	let workState = $state('');
	let memberships = $state<Membership[]>([]);
	let lastPracticeId = $state<string | undefined>();
	// #606: whether *this session* showed a second factor at sign-in --
	// see SessionInfo's own doc comment for why that is not the same
	// question as "is one currently enrolled".
	let hasSecondFactor = $state(false);
	let isLoaded = $state(false);
	let loadError = $state('');

	/*
	 * The way back, at the top of the column (#2044). It was a heading at
	 * the foot of the shell listing every Practice. One Back link names
	 * one place: the Practice she was last in, which is where she came
	 * from; any Practice when she has one; the chooser at `/` when she has
	 * several and none is the last. None at all when she belongs to no
	 * Practice (#1526's founder): a way back to nowhere.
	 */
	const back = $derived.by((): { href: string; label: string } | undefined => {
		const last = memberships.find((membership) => membership.practiceId === lastPracticeId);
		const only = memberships.length === 1 ? memberships[0] : undefined;
		const practice = last ?? only;
		if (practice) {
			return {
				href: resolve('/practices/[practiceId]', { practiceId: practice.practiceId }),
				label: `Back to ${practice.practiceName}`
			};
		}
		if (memberships.length > 1) return { href: resolve('/(signed-out)'), label: 'Back to your Practices' };
		return undefined;
	});

	// #606: voluntary removal of a second factor. The step-up Identity
	// Platform demands of an already-enrolled identity is `ReauthPrompt`'s
	// (#694), so all this screen holds is whether she has asked for it.
	let isRemovingSecondFactor = $state(false);
	const mfaSubmission = new FormSubmission();
	// A refusal that names no field: the network, or the DELETE itself.
	const mfaServiceError = $derived(
		mfaSubmission.errors.length > 0 && !mfaSubmission.errors[0]!.targetId
			? mfaSubmission.errors[0]!.message
			: undefined
	);

	// #615: whether she is the only Owner of some Practice, which is the
	// whole population that ever holds saved recovery codes. Read off the
	// session rather than derived here -- the same session-carried fact
	// `hasSecondFactor` is, and the rotate endpoint re-derives it and
	// refuses on its own regardless (ADR-0006). Since #1531 the section
	// also needs `hasSecondFactor`: before she has one there is nothing
	// for a code to recover, and the rotate endpoint refuses her.
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

		firstName = result.session.firstName;
		lastName = result.session.lastName;
		email = result.session.email;
		reportedAt = result.session.workStateReportedAt;
		workState = workStateName(result.session.workState);
		memberships = result.session.memberships;
		lastPracticeId = result.session.lastPracticeId;
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
	 * show on this screen -- she is signed out and sent to sign in again,
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

		await goto(staffLoginAfterSessionEnded());
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
	 * -- a person asking DoulaCloud, where Erasure is a Client asking her
	 * Practice and Deletion is a Practice asking DoulaCloud.
	 *
	 * It lives here rather than on a Practice screen for the same reason
	 * the work state does: her login is one fact about one person, however
	 * many Practices she works at, and a control for it inside a
	 * per-Practice frame would be telling the same small lie about its own
	 * reach that this whole route exists to avoid.
	 *
	 * Unlike the work state, this one does get a ConfirmDialog, and for
	 * the reason the work-state question page's comment says a
	 * confirmation has to earn: the act is irreversible. What it destroys and what it keeps is
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

{#snippet summary()}
	<!--
		The facts about her, one row each, and on each row the one way it
		changes (#2044). A row names its act through SummaryList's hidden
		text, so a screen reader listing the controls hears "Change, name"
		rather than four bare words.
	-->
	<SummaryList
		isBordered
		rows={[
			{ key: 'name', label: 'Name', value: `${firstName} ${lastName}`, changes: 'name' },
			{
				key: 'work-state',
				label: 'Where you work',
				value: workState || 'Not given yet',
				changes: 'where you work'
			},
			{
				key: 'email',
				label: 'Email address',
				value: email,
				changes: 'to your email address',
				isDetailShown: Boolean(resendNotice || resendError)
			},
			{
				key: 'second-factor',
				label: 'Two-factor authentication',
				value: hasSecondFactor ? 'On' : 'Off',
				changes: 'two-factor authentication',
				isDetailShown: isRemovingSecondFactor || mfaServiceError !== undefined
			}
		]}
	>
		{#snippet value(row)}
			{row.value}
			<!-- The reported date is the only staleness signal the design
			     has (#437), so it stays beside the state it dates. -->
			{#if row.key === 'work-state' && reportedAt}
				<Text text={`Last confirmed ${workStateReportedOn(reportedAt)}.`} step="meta" tone="muted" />
			{/if}
		{/snippet}

		{#snippet action(row, describedBy)}
			{#if row.key === 'name'}
				<Link href={resolve('/(person)/account/name')} label="Change" {describedBy} />
			{:else if row.key === 'work-state'}
				<Link href={resolve('/(person)/account/work-state')} label="Change" {describedBy} />
			{:else if row.key === 'email'}
				<!--
					#613: no verified-email flag is exposed, so this is offered
					whatever the address's state; the outbox worker skips mailing
					an address already verified.
				-->
				<Button
					type="button"
					variant="link"
					label="Send a new verification link"
					loading={isResendingVerification}
					{describedBy}
					onClick={handleResendVerification}
				/>
			{:else if hasSecondFactor}
				<!--
					#606: `hasSecondFactor` is the session-carried fact (see
					SessionInfo's doc comment), the same one staffauth.Middleware
					reads. Removing it asks for the password in place, under the
					row, because ReauthPrompt holds a live Identity Platform
					sign-in that a page of its own would have to hand across a
					navigation.
				-->
				<Button
					type="button"
					variant="link"
					label="Remove"
					expanded={isRemovingSecondFactor}
					{describedBy}
					onClick={isRemovingSecondFactor ? cancelMfaRemoval : beginMfaRemoval}
				/>
			{:else}
				<!--
					A link, not a button: this only ever navigates. `returnTo`
					brings her back here once enrollment finishes.
				-->
				<Link
					href={`${resolve('/(signed-out)/mfa/enroll')}?returnTo=${encodeURIComponent(resolve('/(person)/account'))}`}
					label="Set up"
					{describedBy}
				/>
			{/if}
		{/snippet}

		{#snippet detail(row)}
			{#if row.key === 'email'}
				{#if resendNotice}
					<Notice variant="status" message={resendNotice} />
				{:else}
					<Notice variant="error" message={resendError} />
				{/if}
			{:else}
				<stack-l space="var(--space-4)">
					{#if isRemovingSecondFactor}
						<ReauthPrompt
							idPrefix="account-mfa"
							{email}
							prompt="Confirm your password to remove two-factor authentication."
							confirmLabel="Remove"
							confirmVariant="destructive"
							submission={mfaSubmission}
							onAuthenticated={removeSecondFactor}
							onCancel={cancelMfaRemoval}
						/>
					{/if}
					<!--
						Field-targeted refusals (a wrong password, a wrong code) show
						beside their own control through LabeledField's `error`. A
						service-level failure names no field, so it is the one case
						shown here.
					-->
					{#if mfaServiceError}
						<Notice variant="error" message={mfaServiceError} />
					{/if}
				</stack-l>
			{/if}
		{/snippet}
	</SummaryList>
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
		<ol class="codes prose-list">
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
			text="You are the only owner of a Practice, so nobody else can vouch for you. Recovery codes are how you get back in if you lose your authenticator app."
		/>
		<Text
			text="They are shown once, when you ask for them. DoulaCloud keeps no copy it can read back to you."
			tone="variant"
		/>
		<Button
			type="button"
			label="Show my recovery codes"
			onClick={() => (isConfirmingSavedCodes = true)}
		/>
	{/if}
	<!--
		A `Notice` beside the control rather than an `ErrorSummary`: this
		page asks no question, and a summary at the top of the page linking
		to nothing she can fix would send her away from the thing that
		failed. Nothing here can produce a field-targeted refusal -- there
		is no field -- so the guard is only that a refusal exists at all.
	-->
	{#if savedCodesSubmission.errors.length > 0}
		<Notice variant="error" message={savedCodesSubmission.errors[0].message} />
	{/if}
{/snippet}

{#snippet deleteLoginSection()}
	<!--
		Last and apart (#2044): the one act on this page that cannot be
		undone, so it is nowhere near the everyday rows above (the brief's
		Fitts's Law row). Named in full, and split the way ADR-0033 splits
		it: what goes, and what stays. The second half matters as much as
		the first -- someone leaving a Practice has every reason to fear she
		is taking its record of her work with her, and she is not.
	-->
	<WarningText
		message="Deleting your login ends your access to DoulaCloud everywhere, at once. It cannot be undone."
	/>
	<Text
		text="This deletes your login and your membership of every Practice you work at, and signs you out on every device."
		tone="variant"
	/>
	<Text
		text="Everything you did stays with the Practices you did it for &mdash; the Messages you sent, the Contracts you are named on, the Visits you worked. DoulaCloud keeps its own record that this account existed and that you deleted it."
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
		consequence="Your login, and your membership of every Practice you work at, are deleted immediately. You cannot sign in again. Everything you did stays with those Practices, and DoulaCloud keeps its own record that this account existed."
		confirmLabel="Delete your login"
		error={deleteLoginError}
		onConfirm={handleDeleteLogin}
	/>
{/snippet}

<!--
	A 404 `loadError` means the verified identity has no staff row behind
	it -- signed in, but nobody here yet -- so there is nothing to change,
	and offering a control whose save could never land would be worse than
	saying so. `loading`/`loadError` are `RecordDetail`'s own
	frame-reserving states (#480).
-->
<RecordDetail
	title="Your account"
	{outcome}
	backHref={back?.href}
	backLabel={back?.label}
	isBackPending
	{summary}
	summaryRows={[
		{ label: 'Name', value: 'Given Family-name', action: 'Change' },
		{ label: 'Where you work', value: ['New York', 'Last confirmed'], action: 'Change' },
		{ label: 'Email address', value: 'given.family@example.com', action: 'Send a new verification link' },
		{ label: 'Two-factor authentication', value: 'Off', action: 'Set up' }
	]}
	sections={[
		...(isSoleOwner && hasSecondFactor ? [{ heading: 'Recovery codes', content: savedCodesSection }] : []),
		{ heading: 'Delete your login', content: deleteLoginSection }
	]}
	loading={isLoaded || loadError ? undefined : 'Loading your account'}
	loadError={loadError || undefined}
/>

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

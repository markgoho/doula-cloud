<script lang="ts">
	import {
		createUserWithEmailAndPassword,
		signInWithEmailAndPassword,
		signOut,
		type Auth,
		type UserCredential
	} from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiBaseURL } from '#lib/api.js';
	import { legalVersionUrl, privacyPolicyUrl, termsOfServiceUrl } from '#lib/legal.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import StaffNameFields from '#lib/components/molecules/StaffNameFields.svelte';
	import WorkStateField from '#lib/components/molecules/WorkStateField.svelte';
	import TimezoneField from '#lib/components/molecules/TimezoneField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';
	import { authRefusal, isEmailAlreadyInUse, refusalErrors, refusalOrConfirmable } from '#lib/formErrors.js';
	import { FormSubmission, orServiceProblem, type FormError } from '#lib/formSubmission.svelte.js';
	import { workStateCode } from '#lib/workStates.js';
	import { TIMEZONE_HINT, TIMEZONE_NEEDED, detectTimezone } from '#lib/timezones.js';
	import { ROLE_LABELS } from '#lib/roles.js';
	import {
		PASSWORD_HINT,
		PASSWORD_RULES,
		passwordRefusal,
		preloadCommonPasswords
	} from '#lib/passwordRule.js';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	const practiceNameId = 'signup-practice-name';
	const firstNameId = 'signup-first-name';
	const lastNameId = 'signup-last-name';
	const workStateId = 'signup-work-state';
	const timezoneId = 'signup-timezone';
	const emailId = 'signup-email';
	const passwordId = 'signup-password';
	const agreementId = 'signup-agreement';
	const comingId = 'signup-agreement-coming';

	// #1824: the button is described by the sentence, and during a notice
	// period by the list of coming versions after it, so a person who
	// moves to the button hears both, in the order they read.
	const buttonDescription = $derived(data.coming.length > 0 ? `${agreementId} ${comingId}` : agreementId);

	// #290: read off ROLE_LABELS -- the same source MembershipFields draws
	// its checkboxes from -- rather than spelling the three words out here
	// a second time.
	const roleLabels = ROLE_LABELS.map((role) => role.label);
	const roleNames = `${roleLabels.slice(0, -1).join(', ')} and ${roleLabels.at(-1)}`;

	// docs/api-design.md section 7's Details is keyed by the DTO's own
	// JSON field name -- POST /api/staff/signup's body, right below.
	// The Email field is not in here. It is the Identity Platform account's
	// address, and the BFF never sees it: `POST /api/staff/signup` reads
	// the address off the verified ID token rather than off the body
	// (#614), so it has no body field to key a Details entry to.
	const signupFieldIds = {
		firstName: firstNameId,
		lastName: lastNameId,
		practiceName: practiceNameId,
		workState: workStateId,
		timezone: timezoneId
	};

	let practiceName = $state('');
	let firstName = $state('');
	let lastName = $state('');
	let workStateName = $state('');
	// #1166: opened on the zone the browser reports, which GOV.UK's Select
	// guidance allows for something the service already knows -- she is
	// confirming a reported answer rather than being nudged toward a
	// guess. A browser that reports a zone outside the seven still shows
	// it, rather than rendering a select whose value matches no option
	// (timezoneOptions).
	let timezone = $state(detectTimezone());
	let email = $state('');
	let password = $state('');
	const submission = new FormSubmission();
	// #1539: the list loads while she fills in the form, not on submit.
	preloadCommonPasswords();

	// #816: this multi-step form has no single Continue button to hang
	// #610's cross-population warning on, so it hangs on the form's own
	// final submit -- the deliberate press that mints a session, the same
	// reasoning the sign-in page's Continue button and the portal's own
	// Continue button already use. See the sign-in page's copy of this
	// comment for the fuller picture.
	//
	// #1790's 'pilot-not-open' is the third: until the founder opens the
	// pilot, the BFF refuses an address he has not added, and nothing she
	// typed is wrong -- so it is a screen of its own, not a summary entry.
	let step = $state<'form' | 'confirm-sign-out' | 'pilot-not-open'>('form');

	// The waitlist, tagged so the teaser's visit counter can tell a visit
	// sent from this screen from every other (#368).
	const waitlistURL =
		'https://doula.cloud/?utm_source=app&utm_medium=signup&utm_campaign=pilot-not-open';
	let signOutWarning = $state<string | undefined>();
	// The ID token this signup runs on, kept across the confirm press --
	// a plain local, not $state, for the same reason the sign-in page's
	// own pendingIdToken is: the markup never reads it.
	let pendingIdToken = '';

	/*
	 * Every message starts with the field's own noun and says what to do,
	 * which is GOV.UK's rule and is why none of them contains "required".
	 * The order is the order of the fields, because the summary is read
	 * top to bottom and its entries have to match what she sees below it.
	 *
	 * The Practice name has no entry: it is optional, and the BFF gives an
	 * empty one her own name (#1536).
	 */
	async function findRefusals(): Promise<FormError[]> {
		const found: FormError[] = [];
		if (firstName.trim() === '')
			found.push({ message: 'Enter your first name', targetId: firstNameId });
		if (lastName.trim() === '')
			found.push({ message: 'Enter your last name', targetId: lastNameId });
		if (workStateName === '')
			found.push({ message: 'Choose the state you work from', targetId: workStateId });
		if (timezone === '') found.push({ message: TIMEZONE_NEEDED, targetId: timezoneId });
		if (email.trim() === '') found.push({ message: 'Enter your email address', targetId: emailId });
		if (password === '') {
			found.push({ message: 'Enter a password', targetId: passwordId });
		} else {
			// #1539: the common-password half of the rule is checked here, in
			// the browser, because Identity Platform has no such list.
			const refusal = await passwordRefusal(password);
			if (refusal) found.push({ message: refusal, targetId: passwordId });
		}
		return found;
	}

	/*
	 * The credential this signup runs on, whether or not the account
	 * already exists (#745).
	 *
	 * Creating the Identity Platform account and creating the Practice
	 * are two steps, and the first can land while the second fails -- the
	 * signup rate limiter's own 403 is the easiest way to see it. Before
	 * this, the second attempt was refused by Identity Platform ("email
	 * already in use") and the person was left holding an account no
	 * Practice pointed at, with nothing to sign in to. So the taken
	 * address is not treated as the end of the road: signing in with the
	 * same password gets the same account back, and the BFF half runs
	 * again against it. `POST /api/staff/signup` is resumable on the
	 * identity for exactly this (`existingStaff` in
	 * api/internal/staffauth/signupresume.go): it finishes what the
	 * account is missing, and refuses outright rather than building a
	 * second Practice for someone who already has one.
	 *
	 * A password that does not match is not a resumable signup at all --
	 * it is somebody else's address -- so the original refusal is what
	 * gets reported, not the sign-in's.
	 */
	async function credentialFor(auth: Auth): Promise<UserCredential> {
		try {
			return await createUserWithEmailAndPassword(auth, email, password);
		} catch (error_) {
			if (!isEmailAlreadyInUse(error_)) throw error_;
			try {
				return await signInWithEmailAndPassword(auth, email, password);
			} catch {
				throw error_;
			}
		}
	}

	/*
	 * The POST every attempt runs, first unconfirmed and then -- if #816's
	 * cross-population check refuses it -- again with X-Confirmed. Split
	 * out of handleSubmit so the confirmed retry re-runs exactly this,
	 * with the same idToken.
	 */
	async function postSignup(idToken: string, isConfirmed: boolean) {
		// A plain, one-off fetch: this token makes one trip and is never
		// carried around the way apiFetchWithSession's cookie is (#150
		// deleted the shared ID-token helper).
		return fetch(`${apiBaseURL()}/api/staff/signup`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
				Authorization: `Bearer ${idToken}`,
				...(isConfirmed && { 'X-Confirmed': 'true' })
			},
			// No address goes up. The one she typed above is what
			// `credentialFor` created the Identity Platform account with, so
			// it is already on the ID token in the Authorization header, and
			// that copy -- not a body field -- is what the BFF writes to
			// staff.email (#614).
			body: JSON.stringify({
				practiceName,
				firstName,
				lastName,
				workState: workStateCode(workStateName),
				timezone
			})
		});
	}

	async function land(response: Response) {
		const created: { practiceId: string } = await response.json();

		// SignupHandler already set the session cookie on its own
		// response (#145) -- just drop the JS SDK credential before
		// landing (#149).
		await signOut(getFirebaseAuth());

		await goto(resolve('/practices/[practiceId]', { practiceId: created.practiceId }));
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();

		await submission.run(async () => {
			const refusals = await findRefusals();
			if (refusals.length > 0) return refusals;

			const credential = await credentialFor(getFirebaseAuth());
			pendingIdToken = await credential.user.getIdToken();

			const response = await postSignup(pendingIdToken, false);
			if (!response.ok) {
				// #816: a live portal session in this browser is not a form
				// error -- nothing is wrong with what she typed, and the same
				// submit sent again with X-Confirmed goes through. Keep the JS
				// SDK signed in for that retry, exactly as the sign-in page's
				// own confirm-sign-out step does.
				const refusal = await refusalOrConfirmable(response, signupFieldIds);
				if (refusal.kind === 'confirmable') {
					signOutWarning = refusal.message;
					step = 'confirm-sign-out';
					return;
				}
				// Signing out here costs nothing now that the form can pick the
				// same account back up (see `credentialFor`): submitting again
				// signs in with the credential she still holds -- her password --
				// and finishes the half that failed. Keeping the JS SDK session
				// alive instead would be a second, invisible way to be signed in
				// on a screen that shows a refusal (#745).
				await signOut(getFirebaseAuth());
				// #1790: the account she made stays, and once the founder adds
				// her address the same form sent again finishes the signup
				// through `credentialFor`.
				if (refusal.code === 'PILOT_NOT_OPEN') {
					step = 'pilot-not-open';
					return;
				}
				return refusal.errors;
			}

			await land(response);
		}, mapSignupRefusal);
	}

	// Identity Platform refuses an address that already has an account and a
	// password it thinks too weak, and both belong to a field on this page --
	// `authRefusal` is what turns its code into a message and a target
	// (#467). `findRefusals`'s own array passes straight through.
	function mapSignupRefusal(refusal: unknown): FormError[] {
		return Array.isArray(refusal) ? refusal : [authRefusal(refusal, { emailId, passwordId })];
	}

	/*
	 * #816's press-through: the same signup, sent again with X-Confirmed,
	 * on the same Identity Platform credential the first attempt already
	 * produced.
	 */
	async function handleConfirmSignOut() {
		await submission.run(async () => {
			const response = await postSignup(pendingIdToken, true);
			if (!response.ok) {
				const refusal = await refusalErrors(response, signupFieldIds);
				step = 'form';
				await signOut(getFirebaseAuth());
				return refusal;
			}
			await land(response);
		}, orServiceProblem);
	}

	/*
	 * Backing out of #816's warning. Nothing was minted, so the only way
	 * back in is to submit again -- and the portal session she chose to
	 * keep is untouched. The Identity Platform account this attempt
	 * created or signed into is left as-is: `credentialFor` picks it back
	 * up on the next submit (#745).
	 */
	async function handleCancelSignOut() {
		signOutWarning = undefined;
		pendingIdToken = '';
		step = 'form';
		await signOut(getFirebaseAuth());
	}

	// Moves focus to #1790's first line the moment it replaces the form.
	// It is a paragraph, not a heading, so it cannot take `Heading`'s own
	// `focusOnAppearing` (#2018); the reason is the same as the atom gives.
	function focusOnAppearing(element: HTMLElement) {
		element.focus();
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet content()}
	{#if step === 'confirm-sign-out'}
		<!--
			#816: the warning goes on the button that acts, not on a screen
			of its own -- see the sign-in page's own copy of this pattern.
			Her form is already accepted, so there is nothing to re-render
			here but the consequence and the two ways out of it.
		-->
		<Heading level={2} text="Before you continue" focusOnAppearing />
		<WarningText message={signOutWarning ?? ''} />
		<Button
			type="button"
			label="Continue and sign out"
			loading={submission.isSubmitting}
			onClick={handleConfirmSignOut}
		/>
		<Button type="button" label="Cancel" variant="secondary" onClick={handleCancelSignOut} />
	{:else if step === 'pilot-not-open'}
		<!--
			#1790: in the founder's voice, the teaser's own words for the
			waitlist. The second line is for a pilot doula who typed an
			address other than the one the founder wrote to. The heading is
			the page's own h1 (see `title` below), so the page stops saying
			"Sign up" once signup is closed to this address; focus lands on
			the first line, since the button that held it is gone.
		-->
		<p tabindex="-1" {@attach focusOnAppearing}>
			Until then, only doulas in the pilot can sign up. If you're in the pilot, use the email
			address I wrote to.
		</p>
		<p>
			<Link href={waitlistURL} label="Join the waitlist" /> and I'll write to you when it opens.
		</p>
		<Button
			type="button"
			label="Use a different email address"
			variant="secondary"
			onClick={() => (step = 'form')}
		/>
	{:else}
	<!-- `novalidate`: this page refuses the submit and says so once, at the
	     top, rather than letting the browser's own bubble refuse the first
	     empty field and say nothing about the other four (#467). -->
	<StackedForm onSubmit={handleSubmit}>
		<!--
			#1536 (decided on #1493): her name comes first, because an empty
			Practice name takes it. The hint is two sentences, not the brief's
			one short line, because #1493 decided it has to say both things
			before she types: what an empty field does, and where Clients see
			the name.
		-->
		<StaffNameFields
			bind:firstName
			bind:lastName
			firstId={firstNameId}
			lastId={lastNameId}
			firstError={submission.errorFor(firstNameId)}
			lastError={submission.errorFor(lastNameId)}
		/>
		<LabeledField
			id={practiceNameId}
			label="Practice name (optional)"
			hint="Clients see it on Contracts and card statements. Leave it empty to use your own name."
			error={submission.errorFor(practiceNameId)}
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					value={practiceName}
					onInput={(value) => (practiceName = value)}
					autocomplete="organization"
				/>
			{/snippet}
		</LabeledField>
		<WorkStateField id={workStateId} bind:value={workStateName} error={submission.errorFor(workStateId)} />
		<TimezoneField
			id={timezoneId}
			bind:value={timezone}
			error={submission.errorFor(timezoneId)}
			hint={TIMEZONE_HINT}
		/>
		<LabeledField id={emailId} label="Email" error={submission.errorFor(emailId)}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="email"
					value={email}
					onInput={(value) => (email = value)}
					required
					autocomplete="email"
				/>
			{/snippet}
		</LabeledField>
		<LabeledField
			id={passwordId}
			label="Password"
			hint={PASSWORD_HINT}
			error={submission.errorFor(passwordId)}
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="password"
					value={password}
					onInput={(value) => (password = value)}
					required
					minlength={6}
					autocomplete="new-password"
					passwordrules={PASSWORD_RULES}
				/>
			{/snippet}
		</LabeledField>
		<!--
			#290: stated on the screen itself, before the point of no return,
			GOV.UK's rule for a consequence a person can still act on --
			never content that only arrives after the POST this button sends.
		-->
		<Notice
			variant="info"
			message={`You hold every role — ${roleNames}. Anyone you invite holds only the roles you choose.`}
		/>
		<!--
			#1547 (ADR-0053): the Practice agrees through its Owner by this
			sentence, joined to the button in place and as its accessible
			description -- GOV.UK's Check answers declaration, with no
			checkbox. Each document opens in a new tab so that she keeps the
			form. The BFF records the agreement, with each document's current
			version, in the transaction this button starts.
		-->
		<p id={agreementId}>
			By creating your Practice, you agree to the <Link
				href={termsOfServiceUrl}
				label="Terms of Service"
			/> and the <Link href={privacyPolicyUrl} label="Privacy Policy" />.
		</p>
		<!--
			#1824 (ADR-0053): during the 30 days before a version starts, a
			Practice that signs up agrees to it too, and the BFF records it in
			the same row. This list names each such version by the date it
			starts and links its own page, from the versions the BFF read
			(+page.ts). A list and not a sentence: each link is a box of its
			own (the Link atom is inline-flex), so at 320px a sentence of
			three of them strands its commas and its full stop on lines of
			their own. Outside a notice period there is no list.
		-->
		{#if data.coming.length > 0}
			<div id={comingId}>
				<p>You also agree to:</p>
				<ul class="prose-list">
					{#each data.coming as version (`${version.document}-${version.version}`)}
						<li>
							<Link
								href={legalVersionUrl(version.document, version.version)}
								label={`${version.name} from ${formatCalendarDay(version.version)}`}
							/>
						</li>
					{/each}
				</ul>
			</div>
		{/if}
		<Button
			type="submit"
			label="Create Practice"
			loading={submission.isSubmitting}
			describedBy={buttonDescription}
		/>
	</StackedForm>
	{/if}
{/snippet}

<EntryPage
	title={step === 'pilot-not-open' ? 'DoulaCloud opens in January 2027' : 'Sign up your Practice'}
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	{content}
/>

<style>
	@layer components {
		/* #1824: the coming versions sit apart from each other the way the
		   links under the agreement screen do, so each is its own target. */
		li + li {
			margin-block-start: var(--space-3);
		}
	}
</style>

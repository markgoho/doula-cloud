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
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import WorkStateField from '#lib/components/molecules/WorkStateField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';
	import {
		authRefusal,
		isEmailAlreadyInUse,
		refusalErrors,
		type FormError
	} from '#lib/formErrors.js';
	import { workStateCode } from '#lib/workStates.js';

	const practiceNameId = 'signup-practice-name';
	const staffNameId = 'signup-staff-name';
	const workStateId = 'signup-work-state';
	const emailId = 'signup-email';
	const passwordId = 'signup-password';

	// docs/api-design.md section 7's Details is keyed by the DTO's own
	// JSON field name -- POST /api/staff/signup's body, right below.
	// The Email field is not in here. It is the Identity Platform account's
	// address, and the BFF never sees it: `POST /api/staff/signup` reads
	// the address off the verified ID token rather than off the body
	// (#614), so it has no body field to key a Details entry to.
	const signupFieldIds = {
		practiceName: practiceNameId,
		staffName: staffNameId,
		workState: workStateId
	};

	let practiceName = $state('');
	let staffName = $state('');
	let workStateName = $state('');
	let email = $state('');
	let password = $state('');
	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);

	function errorFor(targetId: string): string | undefined {
		return errors.find((entry) => entry.targetId === targetId)?.message;
	}

	/*
	 * Every message starts with the field's own noun and says what to do,
	 * which is GOV.UK's rule and is why none of them contains "required".
	 * The order is the order of the fields, because the summary is read
	 * top to bottom and its entries have to match what she sees below it.
	 */
	function findRefusals(): FormError[] {
		const found: FormError[] = [];
		if (practiceName.trim() === '')
			found.push({ message: 'Enter the name of your Practice', targetId: practiceNameId });
		if (staffName.trim() === '') found.push({ message: 'Enter your name', targetId: staffNameId });
		if (workStateName === '')
			found.push({ message: 'Choose the state you work from', targetId: workStateId });
		if (email.trim() === '') found.push({ message: 'Enter your email address', targetId: emailId });
		if (password === '') {
			found.push({ message: 'Enter a password', targetId: passwordId });
		} else if (password.length < 6) {
			found.push({ message: 'Password must be 6 characters or more', targetId: passwordId });
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

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errors = [];

		const refusals = findRefusals();
		if (refusals.length > 0) {
			errors = refusals;
			return;
		}

		isSubmitting = true;
		try {
			const credential = await credentialFor(getFirebaseAuth());
			const idToken = await credential.user.getIdToken();

			// A plain, one-off fetch: this token makes one trip and is never
			// carried around the way apiFetchWithSession's cookie is (#150
			// deleted the shared ID-token helper).
			const response = await fetch(`${apiBaseURL()}/api/staff/signup`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${idToken}` },
				// No address goes up. The one she typed above is what
				// `credentialFor` created the Identity Platform account with, so
				// it is already on the ID token in the Authorization header, and
				// that copy -- not a body field -- is what the BFF writes to
				// staff.email (#614).
				body: JSON.stringify({
					practiceName,
					staffName,
					workState: workStateCode(workStateName)
				})
			});
			if (!response.ok) {
				// Signing out here costs nothing now that the form can pick the
				// same account back up (see `credentialFor`): submitting again
				// signs in with the credential she still holds -- her password --
				// and finishes the half that failed. Keeping the JS SDK session
				// alive instead would be a second, invisible way to be signed in
				// on a screen that shows a refusal (#745).
				errors = await refusalErrors(response, signupFieldIds);
				await signOut(getFirebaseAuth());
				return;
			}

			const created: { practiceId: string } = await response.json();

			// SignupHandler already set the session cookie on its own
			// response (#145) -- just drop the JS SDK credential before
			// landing (#149).
			await signOut(getFirebaseAuth());

			await goto(resolve('/practices/[practiceId]', { practiceId: created.practiceId }));
		} catch (error_) {
			// Identity Platform refuses an address that already has an account
			// and a password it thinks too weak, and both belong to a field on
			// this page -- `authRefusal` is what turns its code into a message
			// and a target (#467).
			errors = [authRefusal(error_, { emailId, passwordId })];
		} finally {
			isSubmitting = false;
		}
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

{#snippet content()}
	<!-- `novalidate`: this page refuses the submit and says so once, at the
	     top, rather than letting the browser's own bubble refuse the first
	     empty field and say nothing about the other four (#467). -->
	<form onsubmit={handleSubmit} novalidate>
		<LabeledField id={practiceNameId} label="Practice name" error={errorFor(practiceNameId)}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					value={practiceName}
					onInput={(value) => (practiceName = value)}
					required
					autocomplete="organization"
				/>
			{/snippet}
		</LabeledField>
		<LabeledField id={staffNameId} label="Your name" error={errorFor(staffNameId)}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					value={staffName}
					onInput={(value) => (staffName = value)}
					required
					autocomplete="name"
				/>
			{/snippet}
		</LabeledField>
		<WorkStateField id={workStateId} bind:value={workStateName} error={errorFor(workStateId)} />
		<LabeledField id={emailId} label="Email" error={errorFor(emailId)}>
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
			hint="Must be 6 characters or more"
			error={errorFor(passwordId)}
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
				/>
			{/snippet}
		</LabeledField>
		<Button type="submit" label="Create Practice" loading={isSubmitting} />
	</form>
{/snippet}

<EntryPage
	title="Sign up your Practice"
	errorSummary={errors.length > 0 ? errorSummary : undefined}
	{content}
/>

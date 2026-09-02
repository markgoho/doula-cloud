<script lang="ts">
	import { createUserWithEmailAndPassword, signOut } from 'firebase/auth';
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
	import { authRefusal, refusalMessage, type FormError } from '#lib/formErrors.js';
	import { workStateCode } from '#lib/workStates.js';

	const practiceNameId = 'signup-practice-name';
	const staffNameId = 'signup-staff-name';
	const workStateId = 'signup-work-state';
	const emailId = 'signup-email';
	const passwordId = 'signup-password';

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
			const credential = await createUserWithEmailAndPassword(getFirebaseAuth(), email, password);
			const idToken = await credential.user.getIdToken();

			// A plain, one-off fetch: this token makes one trip and is never
			// carried around the way apiFetchWithSession's cookie is (#150
			// deleted the shared ID-token helper).
			const response = await fetch(`${apiBaseURL()}/api/staff/signup`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${idToken}` },
				body: JSON.stringify({
					practiceName,
					staffName,
					staffEmail: email,
					workState: workStateCode(workStateName)
				})
			});
			if (!response.ok) {
				errors = [{ message: await refusalMessage(response) }];
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

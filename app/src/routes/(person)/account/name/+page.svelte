<script lang="ts">
	/*
	 * Your account's name question (#2044): the Change on the summary's
	 * Name row. One question, its two fields, one button, and back to the
	 * summary with a status Notice that says what was saved (#1710).
	 *
	 * Correcting her name (#1537). Both fields are sent every time and
	 * both are needed, the same as at signup. There is no staff id in the
	 * path or the body: the endpoint only ever writes the caller's own
	 * row, and records who changed it and when.
	 */
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { refusalErrors } from '#lib/formErrors.js';
	import { FormSubmission, orServiceProblem, type FormError } from '#lib/formSubmission.svelte.js';
	import { gotoWithOutcome } from '#lib/outcome.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import StaffNameFields from '#lib/components/molecules/StaffNameFields.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { loadAccountSession, resetAccountSession } from '../../session.svelte.js';

	const firstNameId = 'account-first-name';
	const lastNameId = 'account-last-name';
	const accountHref = resolve('/(person)/account');

	let firstName = $state('');
	let lastName = $state('');
	const submission = new FormSubmission();

	// The names she has now, as the answer to change. Filled only where
	// she has not typed yet, so a slow read never overwrites her typing.
	onMount(async () => {
		const result = await loadAccountSession();
		if (firstName !== '' || lastName !== '' || !result.ok) return;
		firstName = result.session.firstName;
		lastName = result.session.lastName;
	});

	async function saveName(): Promise<FormError[] | undefined> {
		const refusals: FormError[] = [];
		if (firstName.trim() === '') refusals.push({ message: 'Enter your first name', targetId: firstNameId });
		if (lastName.trim() === '') refusals.push({ message: 'Enter your last name', targetId: lastNameId });
		if (refusals.length > 0) return refusals;

		const response = await apiFetchWithSession('/api/staff/name', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ firstName, lastName })
		});
		if (!response.ok) {
			return await refusalErrors(response, { firstName: firstNameId, lastName: lastNameId });
		}

		const saved: { firstName: string; lastName: string } = await response.json();
		// The summary and the bar read the memoized session, so it is
		// forgotten before the way back (see resetAccountSession).
		resetAccountSession();
		await gotoWithOutcome(accountHref, `Your name is ${saved.firstName} ${saved.lastName}.`);
	}

	async function handleSave(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(saveName, orServiceProblem);
	}
</script>

<!-- Passed only while there is a refusal (#1705). -->
{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs, not to arrange anything. -->
<form onsubmit={handleSave} novalidate>
	<QuestionPage
		journey="Changing your name"
		steps={[]}
		backHref={accountHref}
		question={{ as: 'legend', text: 'What is your name?' }}
		errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	>
		{#snippet content()}
			<StaffNameFields
				bind:firstName
				bind:lastName
				firstId={firstNameId}
				lastId={lastNameId}
				firstError={submission.errorFor(firstNameId)}
				lastError={submission.errorFor(lastNameId)}
			/>
		{/snippet}

		{#snippet actions()}
			<Button type="submit" label="Save your name" loading={submission.isSubmitting} />
		{/snippet}
	</QuestionPage>
</form>

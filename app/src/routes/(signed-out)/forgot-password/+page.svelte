<script lang="ts">
	import { apiBaseURL } from '#lib/api.js';
	import { refusalErrors } from '#lib/formErrors.js';
	import { FormSubmission, orServiceProblem } from '#lib/formSubmission.svelte.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';

	const emailId = 'forgot-password-email';

	let email = $state('');
	const submission = new FormSubmission();
	let hasSubmitted = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			if (email.trim() === '') {
				return [{ message: 'Enter your email address', targetId: emailId }];
			}

			const response = await fetch(`${apiBaseURL()}/api/staff/password-reset/request`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email })
			});
			// #168/#613: same response whether or not the address names an
			// account, so this only reads whether the request itself failed.
			if (!response.ok) {
				return await refusalErrors(response, { email: emailId });
			}
			hasSubmitted = true;
		}, orServiceProblem);
	}
</script>

<PageTitle page="Forgot your password?" isError={submission.errors.length > 0} />

<ErrorSummary errors={submission.errors} />

<Heading level={1} variant="page" text="Forgot your password?" />

{#if hasSubmitted}
	<Notice
		variant="status"
		message="If that email address is on an account, we've sent a link to reset your password."
	/>
{:else}
	<StackedForm onSubmit={handleSubmit}>
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
					autocomplete="username"
				/>
			{/snippet}
		</LabeledField>
		<Button type="submit" label="Send reset link" loading={submission.isSubmitting} />
	</StackedForm>
{/if}

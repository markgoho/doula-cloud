<script lang="ts">
	import { apiBaseURL } from '#lib/api.js';
	import { refusalErrors } from '#lib/formErrors.js';
	import { EmailRequestForm } from '#lib/emailRequestForm.svelte.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import EmailAddressField from '#lib/components/molecules/EmailAddressField.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	const emailId = 'forgot-password-email';

	const form = new EmailRequestForm(emailId, async (email) => {
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
	});
	const submission = form.submission;
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet content()}
	{#if form.isSent}
		<Notice
			variant="status"
			message="If that email address is on an account, we've sent a link to reset your password."
		/>
	{:else}
		<StackedForm onSubmit={form.submit}>
			<EmailAddressField id={emailId} value={form.email} onInput={(value) => (form.email = value)} error={submission.errorFor(emailId)} />
			<Button type="submit" label="Send reset link" loading={submission.isSubmitting} />
		</StackedForm>
	{/if}
{/snippet}

<EntryPage
	title="Forgot your password?"
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	{content}
/>

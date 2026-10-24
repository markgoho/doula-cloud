<script lang="ts">
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiBaseURL } from '#lib/api.js';
	import { refusalErrors } from '#lib/formErrors.js';
	import {
		PASSWORD_HINT,
		PASSWORD_RULES,
		passwordRefusal,
		preloadCommonPasswords
	} from '#lib/passwordRule.js';
	import { FormSubmission, orServiceProblem } from '#lib/formSubmission.svelte.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	const token = page.url.searchParams.get('token') ?? '';
	// #1813: the address the link was mailed to (authmail.tokenMailCopy),
	// only so a password manager saves the new password against it. It is
	// not sent anywhere: the token alone names the account to the BFF.
	// #1827: it is in the fragment, which a browser never sends to a
	// server, so it stays out of request logs. The mail escapes it with
	// url.QueryEscape, and URLSearchParams is that escaping's inverse.
	const accountEmail = new URLSearchParams(page.url.hash.slice(1)).get('email') ?? '';
	const passwordId = 'reset-password-new';

	let newPassword = $state('');
	const submission = new FormSubmission();
	// #1539: the list loads while she types, not on submit.
	preloadCommonPasswords();
	let hasSucceeded = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			if (!token) {
				return [{ message: 'This link has expired or was already used -- ask for a new one.' }];
			}
			// #1539: the BFF refuses a common password on this path too
			// (staffauth/reset.go); checking here first saves the round trip.
			const refusal = await passwordRefusal(newPassword);
			if (refusal) return [{ message: refusal, targetId: passwordId }];

			const response = await fetch(`${apiBaseURL()}/api/staff/password-reset`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token, newPassword })
			});
			if (!response.ok) {
				return await refusalErrors(response, { newPassword: passwordId });
			}
			hasSucceeded = true;
		}, orServiceProblem);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet content()}
	{#if hasSucceeded}
		<Notice
			variant="status"
			message="Your password has been reset. Every device you were signed in on has been signed out."
		/>
		<Link href={resolve('/(signed-out)/login')} label="Continue to log in" />
	{:else}
		<StackedForm onSubmit={handleSubmit}>
			{#if accountEmail}
				<!--
					#1813: a password manager saves a new password against the
					field marked "username" in the same form. Chromium's own
					pattern for a form that does not ask for it: a field that
					holds it, not shown. `hidden` keeps it out of sight, out of
					the tab order and out of the accessibility tree.
				-->
				<!-- eslint-disable-next-line svelte/no-restricted-html-elements -- a field nobody sees or types in: TextInput is a visible, labeled control, and this one has no label, no handler and no value but the link's -->
				<input type="email" autocomplete="username" value={accountEmail} hidden />
			{/if}
			<LabeledField
				id={passwordId}
				label="New password"
				hint={PASSWORD_HINT}
				error={submission.errorFor(passwordId)}
			>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="password"
						value={newPassword}
						onInput={(value) => (newPassword = value)}
						required
						autocomplete="new-password"
						passwordrules={PASSWORD_RULES}
					/>
				{/snippet}
			</LabeledField>
			<Button type="submit" label="Reset password" loading={submission.isSubmitting} />
		</StackedForm>
	{/if}
{/snippet}

<EntryPage
	title="Reset your password"
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	{content}
/>

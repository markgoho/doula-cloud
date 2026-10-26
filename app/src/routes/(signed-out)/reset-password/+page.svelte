<script lang="ts">
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { pushState } from '$app/navigation';
	import { apiBaseURL } from '#lib/api.js';
	import { LINK_EXPIRED_OR_USED, refusalErrors } from '#lib/formErrors.js';
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
	import Text from '#lib/components/atoms/Text.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import UnusableLink from '#lib/components/molecules/UnusableLink.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	// Read once, as the screen mounts. A load with no token (a bare
	// /reset-password, or a reload of the success URL below) shows the
	// used-link notice, not the form: without a token the form can do nothing.
	const token = page.url.searchParams.get('token') ?? '';
	// #1813: the address the link was mailed to (authmail.tokenMailCopy),
	// shown as text so the person sees which account this is, and put in
	// the form's hidden username field so a password manager saves the new
	// password against it. It is not sent anywhere: the token alone names
	// the account to the BFF.
	// #1827: it is in the fragment, which a browser never sends to a
	// server, so it stays out of request logs. The mail escapes it with
	// url.QueryEscape, and URLSearchParams is that escaping's inverse.
	const accountEmail = new URLSearchParams(page.url.hash.slice(1)).get('email') ?? '';
	const passwordId = 'reset-password-new';
	/*
	 * #1813: on success the screen changes its URL to this one. Chromium
	 * pairs a password with the form's username only when a form submitted
	 * by JavaScript signals success by navigating, or by changing the URL
	 * with the History API and removing the form ("Password Form Styles that
	 * Chromium Understands"). The real-Chrome walks on 2026-10-24 (#1813,
	 * walk 2 and walk 2 step 3) proved it: with the notice swapped in at
	 * the same URL, Chrome saved the new password with no username, whether
	 * the username field was hidden or visible; with a history push to
	 * `?done=1` on a 2xx, Chrome offered "Update password?" for the account
	 * email.
	 *
	 * `pushState` (shallow routing), not `goto`: it changes the URL, which
	 * is the signal, with no load and no remount, so `hasSucceeded` stays
	 * the component's own state. A push, not a replace, because the push is
	 * what was walked. The path carries no query but `done=1` and no
	 * fragment, so the spent token and the address leave the URL. A reload
	 * of it has no token, so it shows the used-link notice, which is true,
	 * and never a success it cannot know about.
	 */
	const successURL = `${resolve('/(signed-out)/reset-password')}?done=1`;

	let newPassword = $state('');
	const submission = new FormSubmission();
	// #1539: the list loads while she types, not on submit.
	preloadCommonPasswords();
	let hasSucceeded = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
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
			pushState(successURL, {});
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
		<Link href={resolve('/(signed-out)/login')} label="Continue to sign in" />
	{:else if !token}
		<UnusableLink
			message={LINK_EXPIRED_OR_USED}
			next={{ label: 'Ask for a new link', href: resolve('/(signed-out)/forgot-password') }}
		/>
	{:else}
		<StackedForm onSubmit={handleSubmit}>
			{#if accountEmail}
				<!--
					#1813: the account is shown as a line of text, not as a field.
					A read-only field (#1888) looked as if it could be changed,
					and the token, not this address, names the account to the BFF.
					`overflow-wrap` lets a long address wrap at 320px.
				-->
				<div class="account">
					<Text text={`You are resetting the password for ${accountEmail}.`} />
				</div>
				<!--
					#1813: a password manager saves a new password against the
					field marked "username" in the same form. Chromium's own
					pattern for a form that does not ask for it: a field that
					holds it, not shown ("Password Form Styles that Chromium
					Understands" uses display: none). `hidden` keeps it out of
					sight, out of the tab order and out of the accessibility tree.
					Hidden was not the cause of the missing username on the
					2026-10-24 walks: the same URL on success was (see
					`successURL`). Walk 2 step 3 on #1813 proved this hidden field
					with the URL change: Chrome paired the new password with it.
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

<style>
	@layer components {
		.account {
			overflow-wrap: anywhere;
		}
	}
</style>

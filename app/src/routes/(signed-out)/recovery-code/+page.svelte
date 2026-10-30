<script lang="ts">
	/*
	 * Spending a recovery code (#615, screen by #694).
	 *
	 * Everybody who reaches this screen is signed out and locked out, so
	 * it lives on the signed-out routes beside the login screen and never
	 * asks her to sign in first -- she cannot. Identity Platform
	 * challenges for a second factor on every sign-in while one exists,
	 * which is the whole reason a code is spent before a session rather
	 * than inside one.
	 *
	 * Two kinds of code arrive here and the screen does not ask which: one
	 * an Owner vouched for and read out over the phone, and one a sole
	 * Owner wrote down herself. The endpoint tries both.
	 *
	 * #168's rule is enforced server-side and the job here is not to undo
	 * it: an unknown address and a wrong code come back as one sentence,
	 * and this screen shows whatever the BFF said rather than deciding for
	 * itself which of the two it thinks happened. That is also why the
	 * refusal is untargeted -- pointing it at the code field would be
	 * saying "the code is the wrong half", which is exactly the thing the
	 * server declines to tell anyone.
	 */
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetch } from '#lib/api.js';
	import { spendRecoveryCode } from '#lib/mfaRecovery.js';
	import { FormSubmission, orThrownMessage, type FormError } from '#lib/formSubmission.svelte.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	const emailId = 'recovery-email';
	const codeId = 'recovery-code';

	let email = $state('');
	let code = $state('');
	const submission = new FormSubmission();

	function findEmptyFields(): FormError[] {
		const found: FormError[] = [];
		if (email.trim() === '') found.push({ message: 'Enter your email address', targetId: emailId });
		if (code.trim() === '') found.push({ message: 'Enter your recovery code', targetId: codeId });
		return found;
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			const empty = findEmptyFields();
			if (empty.length > 0) return empty;

			await spendRecoveryCode(apiFetch, email.trim(), code.trim());

			/*
			 * No session was minted, deliberately -- the code clears her
			 * authenticator app and nothing else, so the next thing she does
			 * is sign in with her password alone. `codeSpent` is what tells
			 * the login screen to say so, the same shape `sessionEnded=true`
			 * already uses there.
			 */
			await goto(`${resolve('/(signed-out)/login')}?codeSpent=true`);
		}, orThrownMessage);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet content()}
	<!-- Each pair below is one group (#2020): EntryPage spaces its
	     children a group apart, which read two halves of one explanation
	     as two sections. -->
	<stack-l space="var(--space-related)">
		<Text
			text="If you have lost the phone or the app holding your authenticator codes, a recovery code gets you back in."
		/>
		<Text
			text="Use a code you wrote down when you set up your account, or one an owner of your Practice has read out to you."
			tone="variant"
		/>
	</stack-l>
	<StackedForm onSubmit={handleSubmit}>
		<LabeledField id={emailId} label="Email address" error={submission.errorFor(emailId)}>
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
		<!--
			GOV.UK's 30-character step (#1216): the field takes an Owner-vouched
			code of eight digits or a saved code of 26 characters
			(`rand.Text()`), and the longer of the two decides.
		-->
		<LabeledField id={codeId} label="Recovery code" error={submission.errorFor(codeId)} width={30}>
			{#snippet children({ id, describedBy, invalid, width })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					{width}
					value={code}
					onInput={(value) => (code = value)}
					required
					autocomplete="one-time-code"
				/>
			{/snippet}
		</LabeledField>
		<Button type="submit" label="Continue" loading={submission.isSubmitting} />
	</StackedForm>

	<stack-l space="var(--space-related)">
		<Text
			text="No code, and no owner above you? Ask the person who runs your Practice to get in touch with DoulaCloud."
			tone="muted"
			step="body-sm"
		/>
		<Link href={resolve('/(signed-out)/login')} label="Sign in" />
	</stack-l>
{/snippet}

<EntryPage
	title="Use a recovery code"
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	{content}
/>

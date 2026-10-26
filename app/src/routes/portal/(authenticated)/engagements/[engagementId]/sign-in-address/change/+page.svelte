<script lang="ts">
	/*
	 * The sign-in address question (#2045): the Change on the sign-in
	 * address row. One question, its field, "Send the link" under it, and
	 * back to the address with a status Notice (#1710).
	 *
	 * #619, ADR-0026: the change is proved by a link sent to the new
	 * address. Two things about the shape are decisions, not accidents.
	 *
	 * The page never reports whether the address she typed is already in
	 * use. The BFF answers the same way either way (#168's
	 * account-enumeration class), so a screen that said "that one is
	 * taken" would be inventing a fact it was deliberately not told. The
	 * collision surfaces on the confirmation screen instead, where she has
	 * proved the mailbox and so learns nothing new.
	 *
	 * Sending again is not a hazard to design around: asking again
	 * supersedes the first link outright (authtoken.Mint deletes the prior
	 * token and the companion row cascades with it), which is what she
	 * wants if she mistyped the address.
	 */
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { emailFormatError, refusalErrors } from '#lib/formErrors.js';
	import { FormSubmission, orServiceProblem } from '#lib/formSubmission.svelte.js';
	import { gotoWithOutcome } from '#lib/outcome.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';

	const fieldId = 'new-sign-in-address';

	let newAddress = $state('');
	const submission = new FormSubmission();

	const fieldError = $derived(submission.errorFor(fieldId));
	const addressHref = $derived(
		resolve('/portal/(authenticated)/engagements/[engagementId]/sign-in-address', {
			engagementId: page.params.engagementId!
		})
	);

	// The same two checks the BFF makes, made here first so the common
	// mistakes never cost a round trip. The BFF still makes them: this is
	// a convenience, not the enforcement.
	function localError(address: string): string {
		if (address.trim() === '') return 'Enter your new sign-in address';
		return emailFormatError(address.trim()) ?? '';
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			const problem = localError(newAddress);
			if (problem) {
				return [{ message: problem, targetId: fieldId }];
			}

			const sentTo = newAddress.trim();
			const response = await apiFetchWithSession('/api/portal/sign-in-address/request', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email: sentTo })
			});
			if (!response.ok) {
				return await refusalErrors(response, { email: fieldId });
			}
			await gotoWithOutcome(
				addressHref,
				`Check your email. Open the link we sent to ${sentTo} to finish the change. Until you do, your current address still signs you in.`
			);
		}, orServiceProblem);
	}
</script>

<!-- Passed only while there is a refusal (#1705). -->
{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

<!--
	novalidate is GOV.UK's own instruction, not a shortcut past HTML
	validation: the browser's built-in bubble is announced inconsistently,
	disappears on the next keystroke, and cannot be summarized at the top
	of the page. type="email" stays, because it still gets her the right
	keyboard on a phone; what it no longer does is speak for the service.
-->
<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the input, not to arrange anything. -->
<form onsubmit={handleSubmit} novalidate>
	<QuestionPage
		journey="Changing your sign-in address"
		serviceName={page.data.practiceName}
		steps={[]}
		backHref={addressHref}
		question={{ as: 'label', text: 'What is your new sign-in address?', for: fieldId }}
		hint="We will send a link to it. Your current address keeps signing you in until you open the link."
		error={fieldError}
		errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	>
		{#snippet content({ describedBy })}
			<TextInput
				id={fieldId}
				{describedBy}
				invalid={Boolean(fieldError)}
				type="email"
				name="email"
				autocomplete="email"
				value={newAddress}
				onInput={(value) => (newAddress = value)}
			/>
		{/snippet}

		{#snippet actions()}
			<Button type="submit" label="Send the link" loading={submission.isSubmitting} />
		{/snippet}
	</QuestionPage>
</form>

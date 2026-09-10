<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiBaseURL, probeSession } from '#lib/api.js';
	import { decidePortalLanding, type PortalSessionInfo } from '#lib/portalLanding.js';
	import { sessionEndedFrom } from '#lib/sessionEnded.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';
	import { FormSubmission, orServiceProblem } from '#lib/formSubmission.svelte.js';

	const emailId = 'portal-login-email';

	/*
	 * #757: `handleExpiredSession` (#lib/api.js) sends an ended Client
	 * session to this screen flagged as such, exactly as it does the Staff
	 * one, so this screen says why she is here for the same reason. Read
	 * through `#lib/sessionEnded.js` (#1131), the one place that flag is
	 * spelled, so this screen and the writer cannot drift apart.
	 * `$derived` for the reason the Staff screen's own copy of this gives:
	 * `page` is the seam, and a plain read would resolve once at init and
	 * never see a drag-surface override.
	 */
	const hasSessionEnded = $derived(sessionEndedFrom(page.url));

	let email = $state('');
	const submission = new FormSubmission();
	let hasRequested = $state(false);

	// #617: a Client has no password any more, so this screen only ever
	// asks for an address to mail a sign-in link to. The on-load probe is
	// otherwise identical to the Staff login's own (#283): a visitor who
	// already holds a live Client-portal session lands exactly where a
	// fresh sign-in would send her, without the form ever waiting on this
	// to render.
	//
	// #312: with more than one Engagement (or none yet), that is `/` --
	// the app root already probes both populations and renders the same
	// picker `+page.ts` decides from, so this never renders one of its
	// own. A browser holds exactly one Doula Cloud session (#610), so a
	// live portal session here means no Staff session survives to be
	// misrouted by `/`'s own Staff-first probe.
	onMount(async () => {
		const session = await probeSession<PortalSessionInfo>('/api/portal/session');
		if (!session) return;

		const landing = decidePortalLanding(session);
		if (landing.type === 'redirect') {
			await goto(
				resolve('/portal/(authenticated)/engagements/[engagementId]', {
					engagementId: landing.engagementId
				})
			);
		} else {
			await goto(resolve('/(signed-out)'));
		}
	});

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			if (email.trim() === '') {
				return [{ message: 'Enter your email address', targetId: emailId }];
			}

			// The response is identical whether or not the address is on
			// record (#168) -- there is nothing here for a refused submit to
			// report.
			await fetch(`${apiBaseURL()}/api/portal/magic-link/request`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email })
			});
			hasRequested = true;
		}, orServiceProblem);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet content()}
	{#if hasRequested}
		<Notice
			variant="status"
			message="If that address is on our records, we have sent a sign-in link. It can take a minute to arrive."
		/>
	{:else}
		<!--
			#757: why she is looking at this form again. Not rendered beside
			the "we have sent a link" notice above -- once she has asked for
			one, the session that ended is no longer what the screen is
			about, and two stacked notices say less than one.
		-->
		{#if hasSessionEnded}
			<Notice
				variant="info"
				message="For your security, we signed you out. Ask for a new sign-in link to continue."
			/>
		{/if}

		<!-- `novalidate`: this page refuses the submit, not the browser. -->
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
			<Button type="submit" label="Send me a sign-in link" loading={submission.isSubmitting} />
		</StackedForm>
	{/if}
{/snippet}

<EntryPage title="Log in" errorSummary={submission.errors.length > 0 ? errorSummary : undefined} {content} />

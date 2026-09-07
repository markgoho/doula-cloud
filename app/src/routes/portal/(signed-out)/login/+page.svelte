<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiBaseURL, probeSession } from '#lib/api.js';
	import { decidePortalLanding, type Engagement, type PortalSessionInfo } from '#lib/portalLanding.js';
	import { CARE_HEADING, NO_CARE_MESSAGE } from '#lib/clientRegister.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';
	import { FormSubmission, orServiceProblem } from '#lib/formSubmission.svelte.js';

	const emailId = 'portal-login-email';

	let email = $state('');
	const submission = new FormSubmission();
	let hasRequested = $state(false);
	let picker = $state<Engagement[] | undefined>();

	// #617: a Client has no password any more, so this screen only ever
	// asks for an address to mail a sign-in link to. The on-load probe is
	// otherwise identical to the Staff login's own (#283): a visitor who
	// already holds a live Client-portal session lands exactly where a
	// fresh sign-in would send her, without the form ever waiting on this
	// to render.
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
			picker = landing.engagements;
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
		<!-- `novalidate`: this page refuses the submit, not the browser. -->
		<form onsubmit={handleSubmit} novalidate>
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
		</form>
	{/if}

	{#if picker}
		<h2>{CARE_HEADING}</h2>
		{#if picker.length === 0}
			<p>{NO_CARE_MESSAGE}</p>
		{:else}
			<ul>
				{#each picker as engagement (engagement.engagementId)}
					<li>
						<Link
							href={resolve('/portal/(authenticated)/engagements/[engagementId]', {
								engagementId: engagement.engagementId
							})}
							label={engagement.practiceName}
						/>
					</li>
				{/each}
			</ul>
		{/if}
	{/if}
{/snippet}

<EntryPage title="Log in" errorSummary={submission.errors.length > 0 ? errorSummary : undefined} {content} />

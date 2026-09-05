<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiBaseURL, apiFetchWithSession } from '#lib/api.js';
	import { refusalErrors, SERVICE_PROBLEM, type FormError } from '#lib/formErrors.js';
	import { decidePortalLanding, type Engagement, type PortalSessionInfo } from '#lib/portalLanding.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	const token = page.url.searchParams.get('token') ?? '';

	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);
	let picker = $state<Engagement[] | undefined>();

	// #617, ADR-0026: the token is spent on this POST, never on the GET
	// that rendered this page -- a mail client or a security scanner
	// following the link to inspect it must not burn it before she reads
	// the mail.
	async function handleContinue() {
		errors = [];
		isSubmitting = true;
		try {
			const response = await fetch(`${apiBaseURL()}/api/portal/magic-link`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token })
			});
			if (!response.ok) {
				errors = await refusalErrors(response);
				return;
			}

			const sessionResponse = await apiFetchWithSession('/api/portal/session');
			if (!sessionResponse.ok) {
				errors = await refusalErrors(sessionResponse);
				return;
			}
			const session: PortalSessionInfo = await sessionResponse.json();
			const landing = decidePortalLanding(session);
			if (landing.type === 'redirect') {
				await goto(
					resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: landing.engagementId })
				);
			} else {
				picker = landing.engagements;
			}
		} catch {
			errors = [{ message: SERVICE_PROBLEM }];
		} finally {
			isSubmitting = false;
		}
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

{#snippet content()}
	{#if !token}
		<Notice variant="error" message="This link is missing its sign-in code." />
	{:else if picker}
		<h2>Choose an Engagement</h2>
		{#if picker.length === 0}
			<p>You don't have an Engagement yet. Ask your Practice to set one up.</p>
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
	{:else}
		<Button type="button" label="Continue" loading={isSubmitting} onClick={handleContinue} />
	{/if}
{/snippet}

<EntryPage title="Sign in" errorSummary={errors.length > 0 ? errorSummary : undefined} {content} />

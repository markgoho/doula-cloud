<script lang="ts">
	/*
	 * #1549 (ADR-0053): after a material change to the Terms of Service or
	 * the Privacy Policy starts, an Owner of the Practice sees this one
	 * screen before she gets to it, and agrees with one press. The first
	 * Owner who agrees settles it for the Practice. Nobody else is asked:
	 * the layout load sends only a session that carries `agreementDue`
	 * here, and the BFF refuses only an Owner (staffauth.Middleware).
	 *
	 * GOV.UK's Check answers declaration, the same as the sentence above
	 * Create Practice at signup: one sentence the press agrees to, joined
	 * to the button as its `aria-describedby`, and no checkbox. There is
	 * no "Not now" (ADR-0053 rejects it: it turns the screen back into
	 * email only). What the screen does not stand in front of -- export,
	 * deletion and sign out -- is one press away under it (ADR-0048).
	 */
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiBaseURL, apiFetch, apiFetchWithSession } from '#lib/api.js';
	import { apiErrorMessage } from '#lib/apiErrorMessage.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import { legalVersionUrl, privacyPolicyUrl, termsOfServiceUrl } from '#lib/legal.js';
	import { practicePushSubscriptionsPath, unregisterPushSubscription } from '#lib/pushRegistration.js';
	import { hasSecondFactor as checkHasSecondFactor } from '#lib/roles.js';
	import { signOutOfSession } from '#lib/signOut.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import SecondFactorNeeded from '#lib/components/molecules/SecondFactorNeeded.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';
	import type { PracticeSession } from '../+layout.js';

	const session = $derived((page.data as { session: PracticeSession }).session);
	const practiceId = $derived(page.params.practiceId!);
	const due = $derived(session.agreementDue ?? []);
	const terms = $derived(due.find((version) => version.document === 'terms'));
	const privacy = $derived(due.find((version) => version.document === 'privacy'));
	const hasSecondFactor = $derived(checkHasSecondFactor(session));

	const title = $derived.by(() => {
		if (terms && privacy) return 'The Terms of Service and the Privacy Policy have changed';
		if (privacy) return 'The Privacy Policy has changed';
		return 'The Terms of Service have changed';
	});

	// Each link opens the version she agrees to, not whatever the page
	// for the document shows that day; a document that did not change
	// links the page for its version in force.
	const termsUrl = $derived(terms ? legalVersionUrl('terms', terms.version) : termsOfServiceUrl);
	const privacyUrl = $derived(privacy ? legalVersionUrl('privacy', privacy.version) : privacyPolicyUrl);

	const declarationId = 'agreement-declaration';
	let isSubmitting = $state(false);
	let failure = $state('');

	async function agree(event: SubmitEvent) {
		event.preventDefault();
		isSubmitting = true;
		failure = '';
		try {
			const response = await apiFetchWithSession(`/api/practices/${practiceId}/terms-agreement`, { method: 'POST' });
			if (!response.ok) {
				failure = await apiErrorMessage(response);
				return;
			}
			await goto(resolve('/practices/[practiceId]', { practiceId }), { invalidateAll: true });
		} catch {
			failure = 'Your agreement could not be saved. Try again.';
		} finally {
			isSubmitting = false;
		}
	}

	async function signOut() {
		failure = '';
		const outcome = await signOutOfSession({
			unsubscribeURL: practicePushSubscriptionsPath(practiceId),
			fetcher: apiFetch,
			unregisterPush: unregisterPushSubscription
		});
		if (outcome.ok) {
			await goto(resolve('/(signed-out)/login'));
		} else {
			failure = outcome.message;
		}
	}
</script>

{#snippet content()}
	<StackedForm onSubmit={agree}>
		{#if failure}
			<Notice variant="error" message={failure} />
		{/if}
		<stack-l space="var(--space-3)">
			{#each due as version (version.document)}
				<p>
					{version.name}, from {formatCalendarDay(version.version)}: {version.change}
				</p>
			{/each}
		</stack-l>
		<p id={declarationId}>
			By continuing, you agree to the <Link href={termsUrl} label="Terms of Service" /> and the <Link
				href={privacyUrl}
				label="Privacy Policy"
			/> for {session.practiceName}.
		</p>
		<div>
			<Button type="submit" label="Agree and continue" loading={isSubmitting} describedBy={declarationId} />
		</div>
	</StackedForm>
	<section>
		<stack-l space="var(--space-3)">
			<Heading level={2} text="If you do not agree" />
			<p>You can take your Practice's data with you, or delete your Practice.</p>
			<ul>
				<li>
					{#if hasSecondFactor}
						<Link href={`${apiBaseURL()}/api/practices/${practiceId}/export`} label="Export your Practice's data" />
					{:else}
						<SecondFactorNeeded
							message="You need two-factor authentication before you can export your Practice's data."
							returnTo={page.url.pathname}
						/>
					{/if}
				</li>
				<li>
					<Link href={resolve('/practices/[practiceId]/settings/delete', { practiceId })} label="Delete your Practice" />
				</li>
				<li>
					<Button variant="link" label="Sign out" onClick={signOut} />
				</li>
			</ul>
		</stack-l>
	</section>
{/snippet}

<EntryPage {title} {content} />

<style>
	@layer components {
		p {
			margin: 0;
		}

		ul {
			display: flex;
			flex-direction: column;
			gap: var(--space-3);
			margin: 0;
			padding: 0;
			list-style: none;
		}
	}
</style>

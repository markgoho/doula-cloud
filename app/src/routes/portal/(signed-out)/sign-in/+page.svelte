<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiBaseURL, apiFetchWithSession } from '#lib/api.js';
	import {
		isUnusableLink,
		LINK_EXPIRED_OR_USED,
		LINK_NOT_COMPLETE,
		refusalErrors,
		refusalOrConfirmable
	} from '#lib/formErrors.js';
	import { FormSubmission, orServiceProblem } from '#lib/formSubmission.svelte.js';
	import { decidePortalLanding, type PortalSessionInfo } from '#lib/portalLanding.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import UnusableLink from '#lib/components/molecules/UnusableLink.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	const token = page.url.searchParams.get('token') ?? '';
	// #2025: the address the link was mailed to, so the screen can say who
	// is signing in. In the fragment, which no browser sends to a server,
	// the way `reset-password` carries its own (#1813, #1827). A label
	// only: the token alone names the account to the BFF.
	const signInAddress = new URLSearchParams(page.url.hash.slice(1)).get('email') ?? '';
	const lede = signInAddress ? `You are signing in as ${signInAddress}.` : 'Continue to see your care.';

	const submission = new FormSubmission();

	/*
	 * #610: what the BFF said continuing costs, once it has refused an
	 * unconfirmed press. A browser holds exactly one DoulaCloud session,
	 * so a doula who is also a Client loses her Practice session by
	 * signing in here -- she is told before it happens, not after. Set
	 * only by the refusal, so the button below is an ordinary Continue
	 * until the BFF says otherwise.
	 */
	let signOutWarning = $state<string | undefined>();

	// #2025: the BFF refused the link itself -- expired, or used already --
	// so Continue can only be refused again.
	let isLinkUnusable = $state(false);

	// #617, ADR-0026: the token is spent on this POST, never on the GET
	// that rendered this page -- a mail client or a security scanner
	// following the link to inspect it must not burn it before she reads
	// the mail.
	async function handleContinue() {
		await submission.run(async () => {
			const response = await fetch(`${apiBaseURL()}/api/portal/magic-link`, {
				method: 'POST',
				// #610 reads the __session cookie off this request to decide
				// whether continuing evicts a Practice session, and a
				// cross-origin request drops it without this -- which would
				// send the cookie nowhere and evict her with no warning at
				// all. A no-op on the same origin, which is what production
				// serves; the same reasoning the Staff login's own exchange
				// gives.
				credentials: 'include',
				headers: {
					'Content-Type': 'application/json',
					// The BFF refuses the first press and answers with what it
					// costs; this is the second press, which is why nothing
					// sends it until `signOutWarning` has been read.
					...(signOutWarning && { 'X-Confirmed': 'true' })
				},
				body: JSON.stringify({ token })
			});
			if (isUnusableLink(response)) {
				isLinkUnusable = true;
				return;
			}
			if (!response.ok) {
				const refusal = await refusalOrConfirmable(response);
				if (refusal.kind === 'confirmable') {
					signOutWarning = refusal.message;
					return;
				}
				return refusal.errors;
			}

			const sessionResponse = await apiFetchWithSession('/api/portal/session');
			if (!sessionResponse.ok) {
				return await refusalErrors(sessionResponse);
			}
			const session: PortalSessionInfo = await sessionResponse.json();
			const landing = decidePortalLanding(session);
			if (landing.type === 'redirect') {
				await goto(
					resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: landing.engagementId })
				);
			} else {
				// #312: more than one Engagement (or none yet) lands on the
				// app root, the address she can return to for the same
				// list -- see the login screen's own comment on why a live
				// portal session here rules out a Staff session surviving to
				// be misrouted by `/`'s own probe order.
				await goto(resolve('/(signed-out)'));
			}
		}, orServiceProblem);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet content()}
	{#if !token || isLinkUnusable}
		<UnusableLink
			message={token ? LINK_EXPIRED_OR_USED : LINK_NOT_COMPLETE}
			next={{ label: 'Ask for a new sign-in link', href: resolve('/portal/(signed-out)/login') }}
			isFocusedOnAppear={isLinkUnusable}
		/>
	{:else}
		<!--
			#610: the warning goes on the Continue button, not on a screen of
			its own -- this button is already a deliberate press on a request
			the BFF handles, which is what makes it the right place. The
			label changes with it, so the action names what it does (GOV.UK's
			own rule for a button that carries a consequence).
		-->
		{#if signOutWarning}
			<WarningText message={signOutWarning} />
		{/if}
		<Button
			type="button"
			label={signOutWarning ? 'Continue and sign out' : 'Continue'}
			loading={submission.isSubmitting}
			onClick={handleContinue}
		/>
	{/if}
{/snippet}

<EntryPage
	title="Sign in"
	lede={token && !isLinkUnusable ? lede : undefined}
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	{content}
/>

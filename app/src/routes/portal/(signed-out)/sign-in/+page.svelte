<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import {
		isUnusableLink,
		LINK_EXPIRED_OR_USED,
		LINK_NOT_COMPLETE,
		refusalOrConfirmable
	} from '#lib/formErrors.js';
	import { FormSubmission, orServiceProblem } from '#lib/formSubmission.svelte.js';
	import { landInPortal, postContinue } from '#lib/portalContinue.js';
	import { sameOriginPath, withPriorLink } from '#lib/returnTo.js';
	import ContinueWithWarning from '#lib/components/molecules/ContinueWithWarning.svelte';
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
			const response = await postContinue('/api/portal/magic-link', { token }, !!signOutWarning);
			if (await isUnusableLink(response)) {
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

			// #2496: the page she opened signed out, bound to this link by the BFF.
			const redeemed: { returnTo?: string } = await response.json();
			const returnTo = sameOriginPath(redeemed.returnTo, page.url.origin);
			if (returnTo) {
				try {
					await goto(returnTo);
					return;
				} catch {
					// No route answers it; she lands as if there were none.
				}
			}

			return await landInPortal();
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
			next={{
				label: 'Ask for a new sign-in link',
				href: token
					? withPriorLink(resolve('/portal/(signed-out)/login'), token)
					: resolve('/portal/(signed-out)/login')
			}}
			isFocusedOnAppear={isLinkUnusable}
		/>
	{:else}
		<ContinueWithWarning warning={signOutWarning} loading={submission.isSubmitting} onClick={handleContinue} />
	{/if}
{/snippet}

<EntryPage
	title="Sign in"
	lede={token && !isLinkUnusable ? lede : undefined}
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	{content}
/>

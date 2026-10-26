<script lang="ts">
	/*
	 * #619, ADR-0026: the second half of a sign-in-address change --
	 * proving the new mailbox.
	 *
	 * In the signed-out group, not the authenticated one, because the link
	 * arrives in the new mailbox and may be opened on a device that has
	 * never signed in. The token is the whole credential, so the screen
	 * needs no session and must not demand one.
	 *
	 * The token is spent on this POST and never on the GET that rendered
	 * the page -- the same scanner rule the sign-in screen keeps: a mail
	 * client following the link to inspect it must not burn it before she
	 * reads the mail.
	 */
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiBaseURL } from '#lib/api.js';
	import { isUnusableLink, LINK_EXPIRED_OR_USED, LINK_NOT_COMPLETE, refusalErrors } from '#lib/formErrors.js';
	import { FormSubmission, orServiceProblem } from '#lib/formSubmission.svelte.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import UnusableLink from '#lib/components/molecules/UnusableLink.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	const token = page.url.searchParams.get('token') ?? '';
	// #2025: the new address, which the link was mailed to, so the screen
	// can say which address is being confirmed. In the fragment, which no
	// browser sends to a server, the way `reset-password` carries its own
	// (#1813, #1827). A label only: the token alone names the change.
	const newAddress = new URLSearchParams(page.url.hash.slice(1)).get('email') ?? '';
	const lede = newAddress
		? `Confirm that you want to sign in with ${newAddress} from now on.`
		: 'Confirm that you want to use this address to sign in to DoulaCloud.';

	// #2025: the BFF refused the link itself -- expired, or used already --
	// so Continue can only be refused again.
	let isLinkUnusable = $state(false);

	const submission = new FormSubmission();
	let confirmedAddress = $state('');

	async function handleContinue() {
		await submission.run(async () => {
			const response = await fetch(`${apiBaseURL()}/api/portal/sign-in-address`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token })
			});
			if (isUnusableLink(response)) {
				isLinkUnusable = true;
				return;
			}
			if (!response.ok) {
				return await refusalErrors(response);
			}
			const body: { signInAddress: string } = await response.json();
			confirmedAddress = body.signInAddress;
		}, orServiceProblem);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet content()}
	{#if !token || isLinkUnusable}
		<!-- #2025: a new address is asked for from the signed-in account screen. -->
		<UnusableLink
			message={token ? LINK_EXPIRED_OR_USED : LINK_NOT_COMPLETE}
			next={{ label: 'Sign in to ask for a new link', href: resolve('/portal/(signed-out)/login') }}
		/>
	{:else if confirmedAddress}
		<Notice variant="status" message="Your sign-in address has changed" />
		<Text text={`From now on you sign in with ${confirmedAddress}. Your old address no longer works.`} />
		<Link href={resolve('/portal/(signed-out)/login')} label="Sign in" />
	{:else}
		<Button type="button" label="Continue" loading={submission.isSubmitting} onClick={handleContinue} />
	{/if}
{/snippet}

<EntryPage
	title="Confirm your sign-in address"
	lede={token && !isLinkUnusable && !confirmedAddress ? lede : undefined}
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	{content}
/>

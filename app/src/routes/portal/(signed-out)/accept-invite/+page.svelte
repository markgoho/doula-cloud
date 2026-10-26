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

	const inviteToken = page.url.searchParams.get('token') ?? '';

	const submission = new FormSubmission();

	/*
	 * #610: what the BFF said continuing costs, once it has refused an
	 * unconfirmed press. ADR-0026 makes the invitation the first magic
	 * link, so this button signs her in and can evict a live Practice
	 * session exactly as a later sign-in link can. See the sign-in page's
	 * own copy of this comment.
	 */
	let signOutWarning = $state<string | undefined>();

	// #2025: the BFF refused the invite itself -- expired, or accepted
	// already -- so Continue can only be refused again.
	let isLinkUnusable = $state(false);

	// #617, ADR-0026: a Client has no password, so accepting an invitation
	// is nothing but pressing Continue -- there is no account-setup step.
	// The token is spent on this POST, never on the GET that rendered this
	// page, the same GET-then-POST shape #617's own sign-in link uses
	// (#610 hangs its cross-tier eviction warning on this same button).
	async function handleContinue() {
		await submission.run(async () => {
			const acceptResponse = await fetch(`${apiBaseURL()}/api/portal/accept-invite`, {
				method: 'POST',
				// See the sign-in page: #610 reads the __session cookie off
				// this request, and a cross-origin request drops it without
				// this.
				credentials: 'include',
				headers: {
					'Content-Type': 'application/json',
					...(signOutWarning && { 'X-Confirmed': 'true' })
				},
				body: JSON.stringify({ inviteToken })
			});
			if (isUnusableLink(acceptResponse)) {
				isLinkUnusable = true;
				return;
			}
			if (!acceptResponse.ok) {
				const refusal = await refusalOrConfirmable(acceptResponse);
				if (refusal.kind === 'confirmable') {
					signOutWarning = refusal.message;
					return;
				}
				return refusal.errors;
			}

			// AcceptInviteHandler already set the session cookie on its own
			// response (#145).
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
				// #312: more than one Engagement -- ADR-0015's own case, a
				// second Practice's invite accepted by an existing Portal
				// Account (#309) -- lands on the app root, the address she
				// can return to for the same list. See the login screen's
				// own comment on why a live portal session here rules out a
				// Staff session surviving to be misrouted by `/`'s probe
				// order.
				await goto(resolve('/(signed-out)'));
			}
		}, orServiceProblem);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet content()}
	<!--
		#2025: only the Practice can send another invite. "Your doula", not
		the Practice's name: the invite mail names no Practice (ADR-0009,
		#221), and a page that spends nothing on its GET (ADR-0026) cannot
		ask whose invite this is.
	-->
	{#if !inviteToken}
		<UnusableLink message={LINK_NOT_COMPLETE} next={{ text: 'Ask your doula to send the invite again.' }} />
	{:else if isLinkUnusable}
		<UnusableLink
			message={LINK_EXPIRED_OR_USED}
			next={{ text: 'Ask your doula to send the invite again.' }}
			isFocusedOnAppear
		/>
	{:else}
		<!-- #610: see the sign-in page for why the warning sits on this button. -->
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
	title="Accept your portal invite"
	lede={inviteToken && !isLinkUnusable ? 'Your doula invited you to see your care online.' : undefined}
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	{content}
/>

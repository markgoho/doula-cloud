<script lang="ts">
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetch } from '#lib/api.js';
	import {
		isUnusableLink,
		LINK_EXPIRED_OR_USED,
		LINK_NOT_COMPLETE,
		refusalOrConfirmable
	} from '#lib/formErrors.js';
	import { FormSubmission, orServiceProblem } from '#lib/formSubmission.svelte.js';
	import { decidePortalLanding, type PortalSessionInfo } from '#lib/portalLanding.js';
	import { landInPortal, postContinue } from '#lib/portalContinue.js';
	import ContinueWithWarning from '#lib/components/molecules/ContinueWithWarning.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import UnusableLink, { type UnusableLinkNext } from '#lib/components/molecules/UnusableLink.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	const inviteToken = page.url.searchParams.get('token') ?? '';
	const PORTAL_ACCESS_ALREADY_HERE = 'PORTAL_ACCESS_ALREADY_HERE';

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

	// #2279: her login already reaches her care at this Practice, so this
	// invite can never be accepted. The way on is to her care if she is
	// signed in, and to sign in if she is not.
	let alreadyHere = $state<{ message: string; next: UnusableLinkNext }>();

	async function wayOnToHerCare(): Promise<UnusableLinkNext> {
		const sessionResponse = await apiFetch('/api/portal/session');
		if (!sessionResponse.ok) return { label: 'Sign in', href: resolve('/portal/(signed-out)/login') };
		const landing = decidePortalLanding((await sessionResponse.json()) as PortalSessionInfo);
		const href =
			landing.type === 'redirect'
				? resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: landing.engagementId })
				: resolve('/(signed-out)');
		return { label: 'Go to your care', href };
	}

	// #617, ADR-0026: a Client has no password, so accepting an invitation
	// is nothing but pressing Continue -- there is no account-setup step.
	// The token is spent on this POST, never on the GET that rendered this
	// page, the same GET-then-POST shape #617's own sign-in link uses
	// (#610 hangs its cross-tier eviction warning on this same button).
	async function handleContinue() {
		await submission.run(async () => {
			const acceptResponse = await postContinue('/api/portal/accept-invite', { inviteToken }, !!signOutWarning);
			if (await isUnusableLink(acceptResponse)) {
				isLinkUnusable = true;
				return;
			}
			if (!acceptResponse.ok) {
				const refusal = await refusalOrConfirmable(acceptResponse);
				if (refusal.kind === 'confirmable') {
					signOutWarning = refusal.message;
					return;
				}
				if (refusal.code === PORTAL_ACCESS_ALREADY_HERE) {
					alreadyHere = { message: refusal.errors[0]!.message, next: await wayOnToHerCare() };
					return;
				}
				return refusal.errors;
			}

			// AcceptInviteHandler already set the session cookie on its own
			// response (#145).
			return await landInPortal();
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
	{:else if alreadyHere}
		<UnusableLink message={alreadyHere.message} next={alreadyHere.next} isFocusedOnAppear />
	{:else}
		<ContinueWithWarning warning={signOutWarning} loading={submission.isSubmitting} onClick={handleContinue} />
	{/if}
{/snippet}

<EntryPage
	title="Accept your portal invite"
	lede={inviteToken && !isLinkUnusable && !alreadyHere ?'Your doula invited you to see your care online.' : undefined}
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	{content}
/>

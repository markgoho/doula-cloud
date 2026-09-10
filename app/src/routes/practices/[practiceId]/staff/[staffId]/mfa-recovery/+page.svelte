<script lang="ts">
	/*
	 * Owner vouching: one Staff member has lost the phone holding her
	 * second factor, and the Owner of her Practice is the person who can
	 * put her back in (#615, screens by #694).
	 *
	 * The one thing this screen exists to say out loud is where the code
	 * goes. It is mailed to the **Owner's own address**, never to the
	 * locked-out person's -- she is a recovery contact, not somebody
	 * clearing the factor on her behalf, and the whole security of the
	 * arrangement rests on the Owner having satisfied herself, on a call
	 * she made, that she is talking to the right person. An Owner who
	 * assumes the mail went to the doula will conclude it was lost and
	 * press again, so the consequence is stated on the screen and again
	 * beside the button that acts (the same "warning on the button that
	 * acts" shape #610 and #816 use).
	 *
	 * It is a route of its own rather than a control on the roster row
	 * because of what it has to fit: a consequence to read, a
	 * re-authentication with two steps of its own, and an outcome that
	 * says what to do next. A table cell at 320px is not that (ADR-0024).
	 *
	 * The POST is Owner-only server-side and refuses on two further
	 * counts of its own -- `RequireRecentAuth`'s fresh Identity Platform
	 * token, and `RequireConfirmed`'s `X-Confirmed` header. Both are
	 * satisfied here by a person doing a thing, not by a header this page
	 * sets on its own: the token comes from the re-authentication she just
	 * completed, and the header rides the press that follows the warning.
	 */
	import { onMount } from 'svelte';
	import type { User } from 'firebase/auth';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiErrorMessage, apiFetch, apiFetchWithSession } from '#lib/api.js';
	import { isOwner as checkIsOwner } from '#lib/roles.js';
	import { loadStaff, type StaffSummary } from '#lib/staff.js';
	import { vouchForStaff } from '#lib/mfaRecovery.js';
	import type { SessionInfo } from '#lib/landing.js';
	import { FormSubmission } from '#lib/formSubmission.svelte.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import ReauthPrompt from '#lib/components/molecules/ReauthPrompt.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import type { PracticeSession } from '../../../+layout.js';

	const session = $derived((page.data as { session: PracticeSession }).session);
	const isOwner = $derived(checkIsOwner(session));
	const practiceId = $derived(page.params.practiceId!);
	const staffId = $derived(page.params.staffId!);

	let member = $state<StaffSummary | undefined>();
	// The Owner's own address -- the whole point of the screen, so it is
	// read and shown rather than described in the abstract.
	let ownAddress = $state('');
	let isLoaded = $state(false);
	let loadError = $state('');

	/*
	 * Every sentence on this screen names the Staff member rather than
	 * saying "her" (#463's rule, `copy.pronoun.usage.spec.ts`), and there
	 * are seven of them, so the name is derived once. The fallback is only
	 * ever seen in the instant between mount and the roster arriving.
	 */
	const memberName = $derived(member?.name ?? 'this Staff member');

	let step = $state<'intro' | 'reauth' | 'sent'>('intro');
	const submission = new FormSubmission();

	onMount(async () => {
		// Owner-only, both endpoints below and the vouch itself, so a
		// non-Owner is never asked for either -- she meets the notice this
		// screen renders instead, not three 403s in a row.
		if (!isOwner) {
			isLoaded = true;
			return;
		}

		try {
			/*
			 * Two independent facts -- who this screen is about, and the
			 * address the code will actually arrive at -- so they are asked
			 * for together rather than one after the other. Neither read
			 * decides whether the other is worth making.
			 */
			const [roster, response] = await Promise.all([
				loadStaff(apiFetchWithSession, practiceId),
				apiFetchWithSession('/api/staff/session')
			]);

			member = roster.members.find((entry) => entry.staffId === staffId);
			if (!member) {
				loadError = 'That Staff member is not on this practice roster.';
				return;
			}

			if (!response.ok) {
				loadError = await apiErrorMessage(response);
				return;
			}
			const own: SessionInfo = await response.json();
			ownAddress = own.email;
			isLoaded = true;
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load this Staff member';
		}
	});

	/*
	 * `apiFetch`, never `apiFetchWithSession`: a step-up token past its
	 * five-minute window is refused with 401, and `apiFetchWithSession`
	 * reads any 401 as a dead session and signs her out of one that is
	 * perfectly alive. `ReauthPrompt` shows the refusal and puts her back
	 * on the password step instead, where starting the step-up over is
	 * the actual fix.
	 */
	async function sendVouchedCode(user: User): Promise<void> {
		await vouchForStaff(apiFetch, practiceId, staffId, await user.getIdToken());
		step = 'sent';
	}

	const rosterHref = $derived(
		resolve('/practices/[practiceId]/staff', { practiceId })
	);
</script>

{#snippet intro()}
	<Text
		text="Use this when a Staff member has lost the phone or the app holding the authenticator codes, and cannot sign in."
		tone="variant"
		measure
	/>
{/snippet}

{#snippet body()}
	{#if !isOwner}
		<Notice
			variant="info"
			message="Only a practice owner can send a recovery code. Ask an owner of this practice to do it."
		/>
	{:else if step === 'sent'}
		<Notice variant="status" message={`We've sent a recovery code to ${ownAddress}.`} />
		<Text
			text={`Read the code out to ${memberName} on a call where you are sure who you are talking to. It works once, and stops working after 24 hours.`}
		/>
		<Text
			text={`${memberName} enters it on the log-in screen, under the link for a lost authenticator app. That switches two-factor authentication off for that account, and ${memberName} can then log in with a password and set up a new authenticator app.`}
			tone="variant"
		/>
		<Link href={rosterHref} label="Staff" />
	{:else if step === 'reauth'}
		<!--
			The warning sits on the act, not on a screen of its own: she has
			already read what this does, and what she needs in front of her
			while she presses is where the code lands.
		-->
		<WarningText
			message={`The code comes to you, at ${ownAddress}. ${memberName} will not receive it, so you will need to pass it on yourself.`}
		/>
		<ReauthPrompt
			idPrefix="vouch"
			email={ownAddress}
			prompt="Confirm your password to send the code."
			confirmLabel="Send the code"
			{submission}
			onAuthenticated={sendVouchedCode}
			onCancel={() => (step = 'intro')}
		/>
	{:else}
		<Text text={`${memberName} works at this practice as ${member?.email}.`} />
		<Text
			text={`Sending a recovery code emails it to you, at ${ownAddress}. It does not go to ${memberName}, and on its own it changes nothing about that account.`}
		/>
		<Text
			text={`You pass the code on to ${memberName} on a call where you are sure who you are talking to. Entering the code is what switches two-factor authentication off, so a new authenticator app can be set up.`}
			tone="variant"
		/>
	{/if}
{/snippet}

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet actions()}
	{#if isOwner && isLoaded && step === 'intro'}
		<Button type="button" label="Send a recovery code" onClick={() => (step = 'reauth')} />
		<Link href={rosterHref} label="Staff" />
	{/if}
{/snippet}

<FormPage
	title={member ? `Help ${member.name} sign in again` : 'Help someone sign in again'}
	{intro}
	fieldsets={isLoaded ? [{ content: body }] : []}
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	{actions}
	loading={isLoaded || loadError ? undefined : 'Loading this Staff member'}
	{loadError}
/>

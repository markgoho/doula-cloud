<script lang="ts">
	/*
	 * The end of setting up a Practice (#2049): what she answered, a way
	 * back to each page, the agreement, and the one button that creates
	 * the Practice (GOV.UK's Check answers pattern). Nothing is sent before
	 * this button, so every check the pages made runs again here: a reload
	 * keeps every answer but the password (`signupJourney.svelte.ts`), and
	 * a person can reach this page by its address.
	 *
	 * A refusal, the browser's or the server's, lands in this page's error
	 * summary, each entry linked to the page that asks the thing refused;
	 * that page then says it beside its own control.
	 */
	import {
		createUserWithEmailAndPassword,
		signInWithEmailAndPassword,
		signOut,
		type Auth,
		type UserCredential
	} from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiBaseURL } from '#lib/api.js';
	import { legalVersionUrl, privacyPolicyUrl, termsOfServiceUrl } from '#lib/legal.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import CheckAnswers, { type AnswerSection } from '#lib/components/templates/CheckAnswers.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';
	import { authRefusal, isEmailAlreadyInUse, refusalErrors, refusalOrConfirmable } from '#lib/formErrors.js';
	import { FormSubmission, orServiceProblem, type FormError } from '#lib/formSubmission.svelte.js';
	import { NOT_ANSWERED } from '#lib/intakeAnswers.js';
	import { timezoneOptions } from '#lib/timezones.js';
	import { workStateCode } from '#lib/workStates.js';
	import {
		BFF_FIELD_IDS,
		FIELD_IDS,
		JOURNEY_CAPTION,
		JOURNEY_LABEL,
		SIGNUP_STEPS,
		changeHref,
		linkedToSteps,
		railSteps,
		signupDraft,
		whereRefusals,
		youRefusals
	} from '../signupJourney.svelte.js';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	const agreementId = 'signup-agreement';
	const comingId = 'signup-agreement-coming';

	// #1824: the button is described by the sentence, and during a notice
	// period by the list of coming versions after it, so a person who
	// moves to the button hears both, in the order they read.
	const buttonDescription = $derived(data.coming.length > 0 ? `${agreementId} ${comingId}` : agreementId);

	const answers = $derived(signupDraft.answers);
	const steps = $derived(railSteps(undefined, signupDraft.visited));
	const ownName = $derived(`${answers.firstName.trim()} ${answers.lastName.trim()}`.trim());
	const timezoneName = $derived(
		timezoneOptions(answers.timezone).find((option) => option.value === answers.timezone)?.label ?? ''
	);

	/*
	 * One group with no heading (#2033): six rows, each a page's answer.
	 * An empty Practice name shows what the BFF does with it (#1536): the
	 * Practice takes her own name.
	 */
	const sections = $derived<AnswerSection[]>([
		{
			answers: [
				{ label: 'Name', value: ownName || NOT_ANSWERED, changeHref: changeHref('you'), changes: 'name' },
				{
					label: 'Email',
					value: answers.email.trim() || NOT_ANSWERED,
					changeHref: changeHref('you'),
					changes: 'email'
				},
				{
					label: 'Practice name',
					value: answers.practiceName.trim() || ownName || 'Your own name',
					changeHref: changeHref('practice'),
					changes: 'Practice name'
				},
				{
					label: 'State you work from',
					value: answers.workStateName || NOT_ANSWERED,
					changeHref: changeHref('where'),
					changes: 'state you work from'
				},
				{
					label: 'Timezone',
					value: timezoneName || NOT_ANSWERED,
					changeHref: changeHref('where'),
					changes: 'timezone'
				}
			]
		}
	]);

	const submission = new FormSubmission();

	// #816: a live portal session in this browser is not a refusal, and
	// #1790's 'pilot-not-open' is a screen of its own: until the founder
	// opens the pilot, the BFF refuses an address he has not added, and
	// nothing she typed is wrong.
	let step = $state<'check' | 'confirm-sign-out' | 'pilot-not-open'>('check');

	// The waitlist, tagged so the teaser's visit counter can tell a visit
	// sent from this screen from every other (#368).
	const waitlistURL =
		'https://doula.cloud/?utm_source=app&utm_medium=signup&utm_campaign=pilot-not-open';
	let signOutWarning = $state<string | undefined>();
	// The ID token this signup runs on, kept across the confirm press. A
	// plain local: the markup never reads it.
	let pendingIdToken = '';

	/*
	 * The credential this signup runs on, whether or not the account
	 * already exists (#745).
	 *
	 * Creating the Identity Platform account and creating the Practice
	 * are two steps, and the first can land while the second fails. A
	 * taken address is not the end of the road: signing in with the same
	 * password gets the same account back, and `POST /api/staff/signup`
	 * is resumable on the identity for exactly this (`existingStaff` in
	 * api/internal/staffauth/signupresume.go). A password that does not
	 * match is somebody else's address, so the original refusal is what
	 * gets reported.
	 */
	async function credentialFor(auth: Auth): Promise<UserCredential> {
		try {
			return await createUserWithEmailAndPassword(auth, answers.email, signupDraft.password);
		} catch (error_) {
			if (!isEmailAlreadyInUse(error_)) throw error_;
			try {
				return await signInWithEmailAndPassword(auth, answers.email, signupDraft.password);
			} catch {
				throw error_;
			}
		}
	}

	/*
	 * The POST every attempt runs, first unconfirmed and then, if #816's
	 * cross-population check refuses it, again with X-Confirmed. No
	 * address goes up: the BFF reads it off the ID token (#614).
	 */
	async function postSignup(idToken: string, isConfirmed: boolean) {
		return fetch(`${apiBaseURL()}/api/staff/signup`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
				Authorization: `Bearer ${idToken}`,
				...(isConfirmed && { 'X-Confirmed': 'true' })
			},
			body: JSON.stringify({
				practiceName: answers.practiceName,
				firstName: answers.firstName,
				lastName: answers.lastName,
				workState: workStateCode(answers.workStateName),
				timezone: answers.timezone
			})
		});
	}

	async function land(response: Response) {
		const created: { practiceId: string } = await response.json();
		// SignupHandler already set the session cookie on its own response
		// (#145): drop the JS SDK credential (#149) and the draft.
		await signOut(getFirebaseAuth());
		signupDraft.clear();
		await goto(resolve('/practices/[practiceId]', { practiceId: created.practiceId }));
	}

	/*
	 * Each refusal links to its page, and the page it links to keeps it,
	 * so the box she is sent to says what is wrong with it.
	 */
	function showRefusals() {
		submission.errors = linkedToSteps(submission.errors);
		signupDraft.refusals = submission.errors.filter((error) => error.targetId);
	}

	async function handleCreate() {
		await submission.run(async () => {
			const refusals = [
				...(await youRefusals(answers, signupDraft.password)),
				...whereRefusals(answers)
			];
			if (refusals.length > 0) return refusals;

			const credential = await credentialFor(getFirebaseAuth());
			pendingIdToken = await credential.user.getIdToken();

			const response = await postSignup(pendingIdToken, false);
			if (!response.ok) {
				// #816: nothing is wrong with what she typed, and the same
				// press sent again with X-Confirmed goes through. Keep the JS
				// SDK signed in for that retry.
				const refusal = await refusalOrConfirmable(response, BFF_FIELD_IDS);
				if (refusal.kind === 'confirmable') {
					signOutWarning = refusal.message;
					step = 'confirm-sign-out';
					return;
				}
				// A retry signs in with the password she still holds and
				// finishes the half that failed (#745).
				await signOut(getFirebaseAuth());
				if (refusal.code === 'PILOT_NOT_OPEN') {
					step = 'pilot-not-open';
					return;
				}
				return refusal.errors;
			}

			await land(response);
		}, mapSignupRefusal);
		showRefusals();
	}

	// Identity Platform refuses an address that already has an account and
	// a password it thinks too weak, and both belong to the first page
	// (#467). The journey's own array passes straight through.
	function mapSignupRefusal(refusal: unknown): FormError[] {
		return Array.isArray(refusal)
			? refusal
			: [authRefusal(refusal, { emailId: FIELD_IDS.email, passwordId: FIELD_IDS.password })];
	}

	/*
	#816's press-through: the same signup, sent again with X-Confirmed.
	*/
	async function handleConfirmSignOut() {
		await submission.run(async () => {
			const response = await postSignup(pendingIdToken, true);
			if (!response.ok) {
				const refusal = await refusalErrors(response, BFF_FIELD_IDS);
				step = 'check';
				await signOut(getFirebaseAuth());
				return refusal;
			}
			await land(response);
		}, orServiceProblem);
		showRefusals();
	}

	/*
	 * Backing out of #816's warning. Nothing was minted, and the account
	 * this attempt made is picked back up on the next press (#745).
	 */
	async function handleCancelSignOut() {
		signOutWarning = undefined;
		pendingIdToken = '';
		step = 'check';
		await signOut(getFirebaseAuth());
	}

	// #1790: back to the first page, whose Continue returns here.
	async function useDifferentEmail() {
		step = 'check';
		await goto(changeHref('you'));
	}

	// Moves focus to #1790's first line the moment it replaces the page. A
	// paragraph, not a heading, so it cannot take `Heading`'s own
	// `focusOnAppearing` (#2018).
	function focusOnAppearing(element: HTMLElement) {
		element.focus();
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet declaration()}
	<!--
		#1547 (ADR-0053): the Practice agrees through its Owner by this
		sentence, joined to the button as its accessible description --
		GOV.UK's Check answers declaration, with no checkbox. Each document
		opens in a new tab so she keeps the page. The BFF records the
		agreement, with each document's current version, in the transaction
		this button starts.
	-->
	<p id={agreementId}>
		By creating your Practice, you agree to the <Link
			href={termsOfServiceUrl}
			label="Terms of Service"
		/> and the <Link href={privacyPolicyUrl} label="Privacy Policy" />.
	</p>
	<!--
		#1824 (ADR-0053): during the 30 days before a version starts, a
		Practice that signs up agrees to it too. A list and not a sentence:
		each link is a box of its own, so at 320px a sentence of three would
		strand its commas on lines of their own.
	-->
	{#if data.coming.length > 0}
		<div id={comingId}>
			<p>You also agree to:</p>
			<ul class="prose-list">
				{#each data.coming as version (`${version.document}-${version.version}`)}
					<li>
						<Link
							href={legalVersionUrl(version.document, version.version)}
							label={`${version.name} from ${formatCalendarDay(version.version)}`}
						/>
					</li>
				{/each}
			</ul>
		</div>
	{/if}
{/snippet}

{#snippet actions()}
	<Button
		label="Create your Practice"
		loading={submission.isSubmitting}
		describedBy={buttonDescription}
		onClick={handleCreate}
	/>
{/snippet}

{#snippet held()}
	{#if step === 'confirm-sign-out'}
		<!--
			#816: the warning goes on the button that acts, and the two ways
			out of it. The button she pressed is gone, so the heading takes
			focus.
		-->
		<Heading level={2} text="Before you continue" focusOnAppearing />
		<WarningText message={signOutWarning ?? ''} />
		<Button
			type="button"
			label="Continue and sign out"
			loading={submission.isSubmitting}
			onClick={handleConfirmSignOut}
		/>
		<Button type="button" label="Cancel" variant="secondary" onClick={handleCancelSignOut} />
	{:else}
		<!--
			#1790: in the founder's voice, the teaser's own words for the
			waitlist. The heading is the page's h1, so the page stops saying
			"Check your answers" once signup is closed to this address.
		-->
		<p tabindex="-1" {@attach focusOnAppearing}>
			Until then, only doulas in the pilot can sign up. If you're in the pilot, use the email
			address I wrote to.
		</p>
		<p>
			<Link href={waitlistURL} label="Join the waitlist" /> and I'll write to you when it opens.
		</p>
		<Button
			type="button"
			label="Use a different email address"
			variant="secondary"
			onClick={useDifferentEmail}
		/>
	{/if}
{/snippet}

{#if step === 'check'}
	<CheckAnswers
		journey={JOURNEY_LABEL}
		{steps}
		backHref={SIGNUP_STEPS.at(-1)!.href}
		caption={JOURNEY_CAPTION}
		title="Check your answers"
		{sections}
		errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
		{declaration}
		{actions}
	/>
{:else}
	<EntryPage
		title={step === 'pilot-not-open' ? 'DoulaCloud opens in January 2027' : 'Check your answers'}
		errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
		content={held}
	/>
{/if}

<style>
	@layer components {
		/* #1824: each coming version is its own target. */
		li + li {
			margin-block-start: var(--space-3);
		}
	}
</style>

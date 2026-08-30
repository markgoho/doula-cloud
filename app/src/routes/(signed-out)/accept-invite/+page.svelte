<script lang="ts">
	/*
	 * Accepting a Staff Invitation, in two steps rather than one (#437).
	 *
	 * The single-step form this replaces asked for a name and a work state
	 * alongside the credentials, and the server discarded both whenever the
	 * caller already had a Staff account -- a name and a work state are
	 * facts about a person, not about a Membership, so someone joining her
	 * second Practice keeps the ones she already asserted (#316, #415).
	 * The screen had no way to know which case it was in until she had
	 * signed in, so it asked everyone and threw half the answers away. That
	 * is a form lying to the person filling it in.
	 *
	 * Signing in is what tells us. So step one is only the credentials, and
	 * step two is whichever question is left: a new person names herself
	 * and says where she works, and a person already Staff is shown what
	 * she already asserted, read-only, with a link to the one screen that
	 * can change it. Nothing asked is discarded, and nothing discarded is
	 * asked.
	 */
	import {
		createUserWithEmailAndPassword,
		signInWithEmailAndPassword,
		signOut,
		type UserCredential
	} from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiBaseURL, apiFetch, apiFetchWithSession } from '#lib/api.js';
	import { authRefusal, refusalMessage, SERVICE_PROBLEM, type FormError } from '#lib/formErrors.js';
	import { decideLanding, type Membership, type SessionInfo } from '#lib/landing.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import WorkStateField from '#lib/components/molecules/WorkStateField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import { workStateCode, workStateName, workStateReportedOn } from '#lib/workStates.js';

	const modeOptions: { value: 'signup' | 'login'; label: string }[] = [
		{ value: 'signup', label: "I'm new here — create an account" },
		{ value: 'login', label: 'I already have an account — log in' }
	];

	const inviteToken = page.url.searchParams.get('token') ?? '';

	const emailId = 'accept-invite-email';
	const passwordId = 'accept-invite-password';
	const nameId = 'accept-invite-name';
	const workStateId = 'accept-invite-work-state';

	let email = $state('');
	let password = $state('');
	let mode = $state<'signup' | 'login'>('signup');
	// The Invitation carries an address and a Membership, never a name --
	// a person names herself here, but only if she has no Staff account
	// already carrying one.
	let name = $state('');
	let workStateName_ = $state('');
	/*
	 * One array for both steps: they never render at the same time, and
	 * each handler clears it before it starts, so an entry can only ever
	 * describe the form on screen.
	 */
	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);
	let picker = $state<Membership[] | undefined>();

	function errorFor(targetId: string): string | undefined {
		return errors.find((entry) => entry.targetId === targetId)?.message;
	}

	/*
	 * The credential from step one, kept in memory across the two steps.
	 *
	 * It has to be: /api/staff/accept-invite authenticates by Bearer ID
	 * token, not by the session cookie, so step two needs to ask this
	 * credential for a fresh token. Which is why the signOut() that the
	 * login screen fires straight after minting the cookie cannot be
	 * copied here -- it stays where it has always been, after the accept
	 * POST, on both outcomes.
	 */
	let credential = $state<UserCredential | undefined>();
	// What the session read after sign-in found: an existing Staff record,
	// or nothing (a 404, meaning she is new here). Undefined until step one
	// has run.
	let existing = $state<SessionInfo | undefined>();
	let step = $state<'identify' | 'accept'>('identify');
	/*
	 * No `if (!inviteToken)` guard in here any more: the markup refuses a
	 * link with no token before it renders the first field, so this
	 * handler cannot run without one. Guarding again would be guarding
	 * against a branch that has no way to be reached, and a check nobody
	 * can trip is a check nobody can trust.
	 */
	async function handleIdentify(event: SubmitEvent) {
		event.preventDefault();
		errors = [];

		const refusals: FormError[] = [];
		if (email.trim() === '')
			refusals.push({ message: 'Enter your email address', targetId: emailId });
		if (password === '') {
			refusals.push({ message: 'Enter your password', targetId: passwordId });
		} else if (mode === 'signup' && password.length < 6) {
			// Only on the signup branch: an existing account's password is
			// whatever it already is, and refusing a short one here would
			// lock out anyone who set one before the rule existed.
			refusals.push({ message: 'Password must be 6 characters or more', targetId: passwordId });
		}
		if (refusals.length > 0) {
			errors = refusals;
			return;
		}

		isSubmitting = true;
		try {
			credential =
				mode === 'signup'
					? await createUserWithEmailAndPassword(getFirebaseAuth(), email, password)
					: await signInWithEmailAndPassword(getFirebaseAuth(), email, password);
			const idToken = await credential.user.getIdToken();

			// Exchange the Identity Platform ID token for the session cookie,
			// because the staff session read right below authenticates by
			// cookie (#149). A plain, one-off fetch: this token makes one trip
			// and is never carried around the way apiFetchWithSession's cookie
			// is (#150 deleted the shared ID-token helper).
			const exchangeResponse = await fetch(`${apiBaseURL()}/api/session`, {
				method: 'POST',
				credentials: 'include',
				headers: { Authorization: `Bearer ${idToken}` }
			});
			if (!exchangeResponse.ok) {
				errors = [{ message: await refusalMessage(exchangeResponse) }];
				await signOut(getFirebaseAuth());
				return;
			}

			/*
			 * apiFetch, not apiFetchWithSession: a 401 here means the cookie
			 * we just minted did not take, and apiFetchWithSession would
			 * answer that by navigating to the login screen -- throwing away
			 * the invite token in the URL, which is the one thing on this
			 * page that cannot be got back. An error she can read and retry
			 * in place is the better failure.
			 */
			const sessionResponse = await apiFetch('/api/staff/session');
			if (sessionResponse.ok) {
				// She is Staff somewhere already. Step two shows her what she
				// asserted rather than asking for it again.
				existing = (await sessionResponse.json()) as SessionInfo;
			} else if (sessionResponse.status === 404) {
				// No staff row behind the verified identity: she is new here,
				// and step two is the two questions only she can answer.
				existing = undefined;
			} else {
				errors = [{ message: await refusalMessage(sessionResponse) }];
				await signOut(getFirebaseAuth());
				return;
			}
			step = 'accept';
		} catch (error_) {
			// Identity Platform's own words name a product and carry a banned
			// adjective. `authRefusal` covers both modes this form handles --
			// "already has an account" on the signup branch is the one it was
			// worth mapping, since this screen offers the other branch right
			// underneath (#467).
			errors = [authRefusal(error_, { emailId, passwordId })];
		} finally {
			isSubmitting = false;
		}
	}

	/*
	 * Moves focus to step two's heading the moment step two mounts.
	 *
	 * Replacing the form under a keyboard or screen reader user without
	 * moving her focus leaves her tabbing forward through a page she was
	 * never told had changed -- and on this page the change is the whole
	 * point, since what step two asks depends on what step one found out.
	 * The heading takes tabindex="-1" so it can hold focus without ever
	 * joining the tab order, which is the GOV.UK pattern for exactly this.
	 */
	function focusOnAppearing(element: HTMLElement) {
		element.focus();
	}

	async function handleAccept(event: SubmitEvent) {
		event.preventDefault();
		errors = [];
		picker = undefined;

		// Nothing to check on the existing-Staff branch: it asks no
		// questions, it only shows what she already asserted.
		if (!existing) {
			const refusals: FormError[] = [];
			if (name.trim() === '') refusals.push({ message: 'Enter your name', targetId: nameId });
			if (workStateName_ === '')
				refusals.push({ message: 'Choose the state you work from', targetId: workStateId });
			if (refusals.length > 0) {
				errors = refusals;
				return;
			}
		}

		isSubmitting = true;
		try {
			const idToken = await credential!.user.getIdToken();

			// A plain, one-off fetch: this endpoint reads the Bearer token,
			// not the session cookie, which is why the credential from step
			// one is still in hand.
			const acceptResponse = await fetch(`${apiBaseURL()}/api/staff/accept-invite`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${idToken}` },
				body: JSON.stringify({
					inviteToken,
					// Empty on the existing-Staff branch, which the server
					// already ignores there -- what she asserted before stands,
					// and this screen never had a newer answer to offer.
					name: existing ? '' : name,
					workState: existing ? '' : workStateCode(workStateName_)
				})
			});
			if (!acceptResponse.ok) {
				errors = [{ message: await refusalMessage(acceptResponse) }];
				await signOut(getFirebaseAuth());
				return;
			}

			// AcceptInviteHandler already set the session cookie on its own
			// response (#145) -- no separate exchange needed, just drop the
			// JS SDK credential before the session probe reads the cookie.
			await signOut(getFirebaseAuth());

			const sessionResponse = await apiFetchWithSession('/api/staff/session');
			if (!sessionResponse.ok) {
				errors = [{ message: await refusalMessage(sessionResponse) }];
				return;
			}
			const session: SessionInfo = await sessionResponse.json();
			const landing = decideLanding(session);
			if (landing.type === 'redirect') {
				await goto(resolve('/practices/[practiceId]', { practiceId: landing.practiceId }));
			} else {
				picker = landing.memberships;
			}
		} catch {
			// A throw past validation is the network or the SDK, not an answer
			// on this form, so the entry names no control.
			errors = [{ message: SERVICE_PROBLEM }];
		} finally {
			isSubmitting = false;
		}
	}
</script>


<!--
	One summary above the <h1>, GOV.UK's position, serving whichever of the
	two forms is on screen: they never render together, and each handler
	clears the array before it runs.
-->
<ErrorSummary {errors} />

<h1>Accept your Staff invite</h1>

{#if !inviteToken}
	<Notice variant="error" message="Missing invite token" />
{:else if step === 'identify'}
	<!--
		Step one asks only what tells us who she is. Nothing about her name
		or her work state can be answered honestly yet, because whether we
		need those answers depends on whether she is Staff already, and
		signing in is what settles that.
	-->
	<!-- `novalidate`: the page refuses the submit, not the browser (#467). -->
	<form onsubmit={handleIdentify} novalidate>
		<Text text="First, sign in or create an account with the address your invite was sent to." />
		<LabeledField id={emailId} label="Email" error={errorFor(emailId)}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="email"
					value={email}
					onInput={(value) => (email = value)}
					required
				/>
			{/snippet}
		</LabeledField>
		<LabeledField id={passwordId} label="Password" error={errorFor(passwordId)}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="password"
					value={password}
					onInput={(value) => (password = value)}
					required
					minlength={6}
				/>
			{/snippet}
		</LabeledField>
		<RadioGroup
			legend="Account mode"
			name="mode"
			options={modeOptions}
			value={mode}
			onChange={(value) => (mode = value)}
		/>
		<Button type="submit" label="Continue" loading={isSubmitting} />
	</form>
{:else}
	<form onsubmit={handleAccept} novalidate>
		<h2 tabindex="-1" {@attach focusOnAppearing}>
			{existing ? 'Check your details' : 'Tell us about yourself'}
		</h2>

		{#if existing}
			<!--
				Read as plain text, not as disabled form controls. A disabled
				input still looks like a question, and a question she cannot
				answer reads as a fault in the page rather than as a fact that
				is already settled. These are hers, already recorded, and the
				only honest thing to render is the value itself.
			-->
			<Text text="These come from the Staff account you already have, so we are not asking again." />
			<Text text={existing.name} />
			<Text
				text={`You work from ${workStateName(existing.workState)}, self-reported ${workStateReportedOn(existing.workStateReportedAt)}.`}
			/>
			<!--
				The correction lives on one screen, because the work state is
				one fact about one person however many Practices she works at
				(#437). Pointing at it here rather than reopening the field
				keeps that true.
			-->
			<Link href={resolve('/account')} label="Change where you work" variant="secondary" />
		{:else}
			<LabeledField id={nameId} label="Your name" error={errorFor(nameId)}>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						value={name}
						onInput={(value) => (name = value)}
						required
					/>
				{/snippet}
			</LabeledField>
			<WorkStateField id={workStateId} bind:value={workStateName_} error={errorFor(workStateId)} />
		{/if}

		<Button type="submit" label="Accept invite" loading={isSubmitting} />
	</form>
{/if}

{#if picker}
	<Heading level={2} variant="section" text="Choose a Practice" />
	<ul>
		{#each picker as membership (membership.practiceId)}
			<li>
				<Link
					href={resolve('/practices/[practiceId]', { practiceId: membership.practiceId })}
					label={membership.practiceName}
				/>
			</li>
		{/each}
	</ul>
{/if}

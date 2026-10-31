<script lang="ts">
	/*
	 * Accepting a Staff Invitation, on one screen (#2238).
	 *
	 * The screen reads its own Invitation first, by the token in the link:
	 * the Practice, the address it was sent to, what she joins as, and
	 * whether that address already signs in to DoulaCloud. So it never asks
	 * which of the two she is, and never asks for the address. A person new
	 * here names herself, says where she works and creates a password; a
	 * person who already has an account types her password and nothing
	 * else, because her name and work state are facts about her, already
	 * recorded (#415, #437).
	 *
	 * The one exception is an account with no Staff record behind it,
	 * which an abandoned acceptance can leave: the server refuses for want
	 * of a name, and the same screen then asks for the name and work state.
	 */
	import {
		createUserWithEmailAndPassword,
		signInWithEmailAndPassword,
		signOut,
		type UserCredential
	} from 'firebase/auth';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiBaseURL, apiFetch, apiFetchWithSession } from '#lib/api.js';
	import {
		authRefusal,
		LINK_EXPIRED_OR_USED,
		LINK_NOT_COMPLETE,
		refusalErrors,
		refusalOrConfirmable
	} from '#lib/formErrors.js';
	import {
		PASSWORD_HINT,
		PASSWORD_RULES,
		passwordRefusal,
		preloadCommonPasswords
	} from '#lib/passwordRule.js';
	import { FormSubmission, orServiceProblem, type FormError } from '#lib/formSubmission.svelte.js';
	import type { SessionInfo } from '#lib/landing.js';
	import { gotoWithOutcome } from '#lib/outcome.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import StaffNameFields from '#lib/components/molecules/StaffNameFields.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import WorkStateField from '#lib/components/molecules/WorkStateField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import UnusableLink from '#lib/components/molecules/UnusableLink.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';
	import { workStateCode, workStateName } from '#lib/workStates.js';
	import { privacyPolicyUrl } from '#lib/legal.js';
	import { asRoles, joinedSentence } from './joined.js';

	interface Invitation {
		practiceName: string;
		address: string;
		roles: string[];
		employmentType: string;
		hasAccount: boolean;
	}

	interface Accepted {
		practiceId: string;
		practiceName: string;
		roles: string[];
		employmentType: string;
	}

	const inviteToken = page.url.searchParams.get('token') ?? '';

	const passwordId = 'accept-invite-password';
	const firstNameId = 'accept-invite-first-name';
	const lastNameId = 'accept-invite-last-name';
	const workStateId = 'accept-invite-work-state';
	// docs/api-design.md section 7's Details is keyed by the DTO's own
	// JSON field name -- POST /api/staff/accept-invite's body, below.
	const acceptFieldIds = { firstName: firstNameId, lastName: lastNameId, workState: workStateId };

	let invitation = $state<Invitation | undefined>();
	let lookup = $state<'loading' | 'ready' | 'unusable' | 'failed'>(inviteToken ? 'loading' : 'unusable');
	let lookupErrors = $state<FormError[]>([]);

	let password = $state('');
	let firstName = $state('');
	let lastName = $state('');
	let workStateName_ = $state('');
	// An account with no Staff record behind it: the server asked for a
	// name, so the screen asks too.
	let isMissingDetails = $state(false);

	const submission = new FormSubmission();
	preloadCommonPasswords();

	const isNew = $derived(invitation !== undefined && !invitation.hasAccount);
	const isAskingDetails = $derived(isNew || isMissingDetails);

	/*
	 * Kept across a retry on the same screen: accept-invite reads a Bearer
	 * ID token, not the cookie, and an account created on a first press
	 * must not be created again on the second.
	 */
	let credential = $state<UserCredential | undefined>();
	let step = $state<'form' | 'confirm-sign-out'>('form');
	let signOutWarning = $state<string | undefined>();

	async function readInvitation() {
		const response = await apiFetch('/api/staff/accept-invite/lookup', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ inviteToken })
		});
		if (response.ok) {
			invitation = (await response.json()) as Invitation;
			lookup = 'ready';
		} else if (response.status === 404 || response.status === 410) {
			lookup = 'unusable';
		} else {
			lookupErrors = await refusalErrors(response);
			lookup = 'failed';
		}
	}

	async function loadInvitation() {
		lookup = 'loading';
		try {
			await readInvitation();
		} catch (error) {
			lookupErrors = orServiceProblem(error);
			lookup = 'failed';
		}
	}

	if (inviteToken) void loadInvitation();

	function refuseEmpty(): FormError[] {
		const refusals: FormError[] = [];
		if (isAskingDetails) {
			if (firstName.trim() === '') refusals.push({ message: 'Enter your first name', targetId: firstNameId });
			if (lastName.trim() === '') refusals.push({ message: 'Enter your last name', targetId: lastNameId });
			if (workStateName_ === '')
				refusals.push({ message: 'Choose the state you work from', targetId: workStateId });
		}
		if (password === '') refusals.push({ message: 'Enter your password', targetId: passwordId });
		return refusals;
	}

	async function signIn(current: Invitation): Promise<UserCredential> {
		return current.hasAccount
			? signInWithEmailAndPassword(getFirebaseAuth(), current.address, password)
			: createUserWithEmailAndPassword(getFirebaseAuth(), current.address, password);
	}

	async function postAccept(isConfirmed: boolean) {
		const idToken = await credential!.user.getIdToken();
		return fetch(`${apiBaseURL()}/api/staff/accept-invite`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
				Authorization: `Bearer ${idToken}`,
				...(isConfirmed && { 'X-Confirmed': 'true' })
			},
			// Empty when she already has a Staff record, which the server
			// ignores there: what she asserted before stands.
			body: JSON.stringify({
				inviteToken,
				firstName: isAskingDetails ? firstName : '',
				lastName: isAskingDetails ? lastName : '',
				workState: isAskingDetails ? workStateCode(workStateName_) : ''
			})
		});
	}

	async function land(response: Response): Promise<FormError[] | undefined> {
		const accepted = (await response.json()) as Accepted;
		// AcceptInviteHandler set the session cookie on its own response
		// (#145): drop the SDK credential before reading the session.
		await signOut(getFirebaseAuth());
		const sessionResponse = await apiFetchWithSession('/api/staff/session');
		if (!sessionResponse.ok) return await refusalErrors(sessionResponse);
		const session = (await sessionResponse.json()) as SessionInfo;

		// The Practice she just joined, not the one she used last (#2265),
		// and the Overview says once what she joined as (#2239).
		const carriedOver = invitation?.hasAccount && !isMissingDetails;
		const message = joinedSentence({
			practiceName: accepted.practiceName,
			roles: accepted.roles,
			employmentType: accepted.employmentType,
			hasOtherPractices: session.memberships.length > 1,
			workState: carriedOver ? workStateName(session.workState) : undefined
		});
		await gotoWithOutcome(resolve('/practices/[practiceId]', { practiceId: accepted.practiceId }), message);
		return undefined;
	}

	async function handleAccept(event: SubmitEvent) {
		event.preventDefault();
		const current = invitation!;

		await submission.run(async () => {
			const refusals = refuseEmpty();
			if (!credential && !current.hasAccount && refusals.length === 0) {
				const refusal = await passwordRefusal(password);
				if (refusal) refusals.push({ message: refusal, targetId: passwordId });
			}
			if (refusals.length > 0) return refusals;

			if (!credential) {
				try {
					credential = await signIn(current);
				} catch (error) {
					return [authRefusal(error, { emailId: passwordId, passwordId })];
				}
			}

			const acceptResponse = await postAccept(false);
			if (!acceptResponse.ok) {
				// #816: a live portal session in this browser is not a form
				// error; the same press, confirmed, goes through.
				const refusal = await refusalOrConfirmable(acceptResponse, acceptFieldIds);
				if (refusal.kind === 'confirmable') {
					signOutWarning = refusal.message;
					step = 'confirm-sign-out';
					return;
				}
				const isAskingForDetails = refusal.errors.some((error) =>
					[firstNameId, lastNameId, workStateId].includes(error.targetId ?? '')
				);
				if (isAskingForDetails && !isMissingDetails && current.hasAccount) {
					isMissingDetails = true;
					return refusal.errors;
				}
				credential = undefined;
				await signOut(getFirebaseAuth());
				return refusal.errors;
			}

			return await land(acceptResponse);
		}, orServiceProblem);
	}

	async function handleConfirmSignOut() {
		await submission.run(async () => {
			const acceptResponse = await postAccept(true);
			if (!acceptResponse.ok) {
				const refusal = await refusalErrors(acceptResponse, acceptFieldIds);
				step = 'form';
				credential = undefined;
				await signOut(getFirebaseAuth());
				return refusal;
			}
			return await land(acceptResponse);
		}, orServiceProblem);
	}

	function handleCancelSignOut() {
		signOutWarning = undefined;
		step = 'form';
	}

	const title = $derived(invitation ? `Join ${invitation.practiceName}` : 'Accept your invite');
	const lede = $derived(
		invitation && `Your invite to join as ${asRoles(invitation.roles)} was sent to ${invitation.address}.`
	);
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={lookup === 'failed' ? lookupErrors : submission.errors} />
{/snippet}

{#snippet content()}
	{#if !inviteToken}
		<!-- #2025: only the Practice can send another invite. -->
		<UnusableLink message={LINK_NOT_COMPLETE} next={{ text: 'Ask the Practice to send the invite again.' }} />
	{:else if lookup === 'unusable'}
		<UnusableLink message={LINK_EXPIRED_OR_USED} next={{ text: 'Ask the Practice to send the invite again.' }} />
	{:else if lookup === 'loading'}
		<Skeleton variant="text" lines={4} label="Loading your invite" />
	{:else if lookup === 'failed'}
		<Button type="button" label="Try again" onClick={loadInvitation} />
	{:else if step === 'confirm-sign-out'}
		<!--
			#816: the warning goes on the button that acts, not on a screen
			of its own -- see the sign-in page's own copy of this pattern.
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
	{:else if invitation}
		<StackedForm onSubmit={handleAccept}>
			{#if isAskingDetails}
				<StaffNameFields
					bind:firstName
					bind:lastName
					firstId={firstNameId}
					lastId={lastNameId}
					firstError={submission.errorFor(firstNameId)}
					lastError={submission.errorFor(lastNameId)}
				/>
				<WorkStateField id={workStateId} bind:value={workStateName_} error={submission.errorFor(workStateId)} />
			{/if}
			<LabeledField
				id={passwordId}
				label={isNew ? 'Create a password' : 'Password'}
				hint={isNew ? PASSWORD_HINT : undefined}
				error={submission.errorFor(passwordId)}
			>
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
						autocomplete={isNew ? 'new-password' : 'current-password'}
						passwordrules={isNew ? PASSWORD_RULES : undefined}
					/>
				{/snippet}
			</LabeledField>

			<!--
				A notice, not a consent (#1557): a Practice agrees through its
				Owner (ADR-0053), so nothing is recorded for it.
			-->
			<p>
				The <Link href={privacyPolicyUrl} label="Privacy Policy" /> tells you what DoulaCloud holds
				about you.
			</p>

			<Button type="submit" label="Accept invite" loading={submission.isSubmitting} />
		</StackedForm>
	{/if}
{/snippet}

<EntryPage
	{title}
	lede={lede || undefined}
	errorSummary={lookup === 'failed' || submission.errors.length > 0 ? errorSummary : undefined}
	{content}
/>

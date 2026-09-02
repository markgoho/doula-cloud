<script lang="ts">
	import { signInWithEmailAndPassword, signOut } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiBaseURL, apiFetchWithSession } from '#lib/api.js';
	import { decideLanding, type Membership, type SessionInfo } from '#lib/landing.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import { authRefusal, refusalMessage, type FormError } from '#lib/formErrors.js';

	const emailId = 'login-email';
	const passwordId = 'login-password';

	let email = $state('');
	let password = $state('');
	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);
	let picker = $state<Membership[] | undefined>();

	/*
	 * One array is the whole mechanism (#467). The summary lists it and
	 * each field reads its own entry out of it, so the two wordings cannot
	 * drift -- they are one string rendered twice.
	 */
	function errorFor(targetId: string): string | undefined {
		return errors.find((entry) => entry.targetId === targetId)?.message;
	}

	// GOV.UK's wording rules: what to do, not what is wrong with it.
	// "Enter your email address", never "Email is required".
	function findEmptyFields(): FormError[] {
		const found: FormError[] = [];
		if (email.trim() === '') found.push({ message: 'Enter your email address', targetId: emailId });
		if (password === '') found.push({ message: 'Enter your password', targetId: passwordId });
		return found;
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		// Cleared first, so a second refused submit unmounts the summary and
		// remounts it -- which is what announces the new failure to anyone
		// who has already tabbed away from the old one.
		errors = [];
		picker = undefined;

		const empty = findEmptyFields();
		if (empty.length > 0) {
			errors = empty;
			return;
		}

		isSubmitting = true;
		try {
			const credential = await signInWithEmailAndPassword(getFirebaseAuth(), email, password);
			const idToken = await credential.user.getIdToken();

			// Exchange the Identity Platform ID token for the session cookie
			// before signing out of the JS SDK -- the Staff session probe
			// right below this authenticates by cookie, so the exchange must
			// land first (#149). A plain, one-off fetch: this token makes one
			// trip and is never carried around the way apiFetchWithSession's
			// cookie is (#150 deleted the shared ID-token helper).
			// credentials: 'include' because apiBaseURL() may point at another
			// origin, and a cross-origin Set-Cookie is dropped without it --
			// which would leave the exchange reporting success and the cookie
			// never arriving. It is a no-op on the same origin, which is what
			// production serves (the /api/** rewrite), so this costs nothing
			// and matches both apiFetch and the accept-invite exchange.
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
			await signOut(getFirebaseAuth());

			const response = await apiFetchWithSession('/api/staff/session');
			if (!response.ok) {
				errors = [{ message: await refusalMessage(response) }];
				return;
			}

			const session: SessionInfo = await response.json();
			const landing = decideLanding(session);
			if (landing.type === 'redirect') {
				await goto(resolve('/practices/[practiceId]', { practiceId: landing.practiceId }));
			} else {
				picker = landing.memberships;
			}
		} catch (error_) {
			// Identity Platform's own words are a product name, a code and a
			// banned adjective ("Firebase: Error (auth/invalid-credential)."),
			// and the flat "Login failed" that replaced them said nothing
			// about which field to look at. `authRefusal` maps the code onto
			// a message and, where there is one, the control that caused it.
			errors = [authRefusal(error_, { emailId, passwordId })];
		} finally {
			isSubmitting = false;
		}
	}
</script>

<PageTitle page="Log in" isError={errors.length > 0} />

<ErrorSummary {errors} />

<h1>Log in</h1>

<!--
	`novalidate`, per GOV.UK's Recover from validation errors pattern: the
	browser's own bubbles refuse the submit before this page can, they
	vanish on the next keystroke, and they are worded by the browser
	rather than by us. `required` stays on the controls, because it is a
	true statement about the field and assistive technology reads it; what
	it no longer does is block.
-->
<form onsubmit={handleSubmit} novalidate>
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
				autocomplete="username"
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
				autocomplete="current-password"
			/>
		{/snippet}
	</LabeledField>
	<Button type="submit" label="Log in" loading={isSubmitting} />
</form>

<Link href={resolve('/(signed-out)/forgot-password')} label="Forgot your password?" />

{#if picker}
	<h2>Choose a Practice</h2>
	{#if picker.length === 0}
		<p>You don't belong to any Practice yet. Ask an Owner to invite you.</p>
	{:else}
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
{/if}

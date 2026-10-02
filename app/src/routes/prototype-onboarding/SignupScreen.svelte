<script lang="ts">
	/*
	 * PROTOTYPE -- #1496, screen 1: `/signup` as #1493 and #1494 decided it.
	 * Her name first, in two fields (#1537). Practice name optional (#1536).
	 * A password of 15 characters or more, no composition rule, no second
	 * field (#1538, #1539). The role notice (#290), the agreement sentence
	 * directly above the button, no checkbox, and the button stays
	 * "Create Practice" with the sentence as its description (#1547).
	 */
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import WorkStateField from '#lib/components/molecules/WorkStateField.svelte';
	import TimezoneField from '#lib/components/molecules/TimezoneField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';
	import type { FormError } from '#lib/formErrors.js';
	import { TIMEZONE_HINT, TIMEZONE_NEEDED, detectTimezone } from '#lib/timezones.js';
	import { COMMON_PASSWORDS, PASSWORD_MIN, words } from './fixtures.js';
	import Link from '#lib/components/atoms/Link.svelte';
	import { prototype } from './model.svelte.js';

	// The marketing site. An address on another origin makes the Link atom
	// open a new tab and say so; the prototype stops the press itself.
	const SITE = 'https://doulacloud.example';

	const firstNameId = 'signup-first-name';
	const lastNameId = 'signup-last-name';
	const practiceNameId = 'signup-practice-name';
	const workStateId = 'signup-work-state';
	const timezoneId = 'signup-timezone';
	const emailId = 'signup-email';
	const passwordId = 'signup-password';
	const agreementId = 'signup-agreement';

	let firstName = $state('');
	let lastName = $state('');
	let practiceName = $state('');
	let workStateName = $state('');
	let timezone = $state(detectTimezone());
	let email = $state('');
	let password = $state('');
	let errors = $state<FormError[]>([]);

	// "Fill the form" on the prototype bar.
	let filled = 0;
	$effect(() => {
		if (prototype.fill === filled) return;
		filled = prototype.fill;
		({ firstName, lastName, practiceName, timezone, email, password } = prototype.sample);
		workStateName = prototype.sample.workState;
	});

	const errorFor = (id: string) => errors.find((entry) => entry.targetId === id)?.message;

	// The order is the order of the fields (#1536).
	function findRefusals(): FormError[] {
		const found: FormError[] = [];
		if (firstName.trim() === '') found.push({ message: 'Enter your first name', targetId: firstNameId });
		if (lastName.trim() === '') found.push({ message: 'Enter your last name', targetId: lastNameId });
		if (workStateName === '') found.push({ message: 'Choose the state you work from', targetId: workStateId });
		if (timezone === '') found.push({ message: TIMEZONE_NEEDED, targetId: timezoneId });
		if (email.trim() === '') found.push({ message: 'Enter your email address', targetId: emailId });
		if (password === '') found.push({ message: 'Enter a password', targetId: passwordId });
		else if (password.length < PASSWORD_MIN)
			found.push({ message: `Password must be ${PASSWORD_MIN} characters or more`, targetId: passwordId });
		else if (COMMON_PASSWORDS.has(password.toLowerCase()))
			found.push({ message: 'Password is too common. Enter a different password', targetId: passwordId });
		return found;
	}

	function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errors = findRefusals();
		if (errors.length > 0) return;
		prototype.signUp({
			firstName: firstName.trim(),
			lastName: lastName.trim(),
			practiceName,
			workState: workStateName,
			timezone,
			email: email.trim()
		});
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

{#snippet content()}
	<StackedForm onSubmit={handleSubmit}>
		<LabeledField id={firstNameId} label="First name" error={errorFor(firstNameId)}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					value={firstName}
					onInput={(value) => (firstName = value)}
					required
					autocomplete="given-name"
				/>
			{/snippet}
		</LabeledField>
		<LabeledField id={lastNameId} label="Last name" error={errorFor(lastNameId)}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					value={lastName}
					onInput={(value) => (lastName = value)}
					required
					autocomplete="family-name"
				/>
			{/snippet}
		</LabeledField>
		<LabeledField id={practiceNameId} label="Practice name (optional)" hint={words.practiceNameHint}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					value={practiceName}
					onInput={(value) => (practiceName = value)}
					autocomplete="organization"
				/>
			{/snippet}
		</LabeledField>
		<WorkStateField id={workStateId} bind:value={workStateName} error={errorFor(workStateId)} />
		<TimezoneField id={timezoneId} bind:value={timezone} error={errorFor(timezoneId)} hint={TIMEZONE_HINT} />
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
					autocomplete="email"
				/>
			{/snippet}
		</LabeledField>
		<LabeledField id={passwordId} label="Password" hint={words.passwordHint} error={errorFor(passwordId)}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="password"
					value={password}
					onInput={(value) => (password = value)}
					required
					minlength={PASSWORD_MIN}
					autocomplete="new-password"
				/>
			{/snippet}
		</LabeledField>
		<Notice variant="info" message={words.roleNotice} />
		<!-- #1547: the sentence, two links that open in a new tab, no checkbox. -->
		<p class="agreement" id={agreementId}>
			By creating your Practice, you agree to the
			<Link href={`${SITE}/terms`} label="Terms of Service" />
			and the
			<Link href={`${SITE}/privacy`} label="Privacy Policy" />.
		</p>
		<Button type="submit" label="Create Practice" describedBy={agreementId} />
	</StackedForm>
{/snippet}

<EntryPage title="Sign up your Practice" errorSummary={errors.length > 0 ? errorSummary : undefined} {content} />

<style>
	.agreement {
		margin: 0;
		max-inline-size: var(--measure);
		font-size: var(--text-body-size);
		line-height: var(--text-body-leading);
		color: var(--color-on-surface);
	}

</style>

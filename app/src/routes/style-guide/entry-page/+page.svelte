<script lang="ts">
	/*
	 * Modelled on `(signed-out)/login`, the plainest of the five real
	 * consumers: a product name already on the bar above, one short
	 * question, two credentials, one button. `picker` demonstrates the
	 * region `content` also has to carry -- the "choose a Practice" list
	 * that appears once a sign-in with more than one Membership succeeds.
	 */
	import EntryPage from '#lib/components/templates/EntryPage.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';

	const emailId = 'style-guide-entry-page-email';
	const passwordId = 'style-guide-entry-page-password';

	let email = $state('anne-marie.ochieng-whitfield@highland-midwifery-group.example.org');
	let password = $state('');
	let hasError = $state(false);
	let isPickerShown = $state(false);

	const noop = () => {};
</script>

{#snippet errorSummary()}
	<ErrorSummary
		errors={[
			{ message: 'Enter your email address', targetId: emailId },
			{ message: 'Enter your password', targetId: passwordId }
		]}
	/>
{/snippet}

{#snippet content()}
	<form onsubmit={(event) => event.preventDefault()} novalidate>
		<LabeledField id={emailId} label="Email" error={hasError ? 'Enter your email address' : undefined}>
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
		<LabeledField
			id={passwordId}
			label="Password"
			error={hasError ? 'Enter your password' : undefined}
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
					autocomplete="current-password"
				/>
			{/snippet}
		</LabeledField>
		<Button type="submit" label="Log in" onClick={noop} />
	</form>

	<Link href="/style-guide/entry-page" label="Forgot your password?" />

	{#if isPickerShown}
		<Heading level={2} variant="section" text="Choose a Practice" />
		<ul>
			<li><Link href="/style-guide/entry-page" label="Highland Midwifery Group" /></li>
			<li><Link href="/style-guide/entry-page" label="Riverside Birth Collective" /></li>
		</ul>
	{/if}
{/snippet}

<div class="controls">
	<Button
		label={hasError ? 'Hide the error state' : 'Show the error state'}
		variant="secondary"
		size="sm"
		onClick={() => (hasError = !hasError)}
	/>
	<Button
		label={isPickerShown ? 'Hide the picker' : 'Show the picker'}
		variant="secondary"
		size="sm"
		onClick={() => (isPickerShown = !isPickerShown)}
	/>
</div>

<EntryPage title="Log in" errorSummary={hasError ? errorSummary : undefined} {content} />

<style>
	@layer components {
		/* Not part of the Template -- switches so the two states can be seen
		   without editing this file. */
		.controls {
			display: flex;
			gap: var(--space-3);
			padding: var(--space-3) var(--space-4);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-container);
		}
	}
</style>

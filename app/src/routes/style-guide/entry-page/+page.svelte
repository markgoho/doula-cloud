<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		hasError?: boolean;
		isPickerShown?: boolean;
	}

	/*
	 * The other states this page renders (#1638, ADR-0025), each its own
	 * subject for the continuum check and the drag surface. The error state
	 * adds the summary and a message below each field; the picker adds the
	 * list `content` carries after a sign-in with more than one Membership.
	 */
	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Entry page, with errors', props: { hasError: true } },
		{ name: 'Entry page, with the Practice picker', props: { isPickerShown: true } }
	];
</script>

<script lang="ts">
	/*
	 * Modeled on `(signed-out)/login`, the plainest of the five real
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
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';

	const emailId = 'style-guide-entry-page-email';
	const passwordId = 'style-guide-entry-page-password';

	let email = $state('anne-marie.ochieng-whitfield@highland-midwifery-group.example.org');
	let password = $state('');

	let { hasError = false, isPickerShown = false }: Properties = $props();

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
	<StackedForm onSubmit={(event) => event.preventDefault()}>
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
	</StackedForm>

	<Link href="/style-guide/entry-page" label="Forgot your password?" />

	{#if isPickerShown}
		<Heading level={2} variant="section" text="Choose a Practice" />
		<ul>
			<li><Link href="/style-guide/entry-page" label="Highland Midwifery Group" /></li>
			<li><Link href="/style-guide/entry-page" label="Riverside Birth Collective" /></li>
		</ul>
	{/if}
{/snippet}

<EntryPage title="Log in" errorSummary={hasError ? errorSummary : undefined} {content} />

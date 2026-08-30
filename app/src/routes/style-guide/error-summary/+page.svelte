<script lang="ts">
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import type { FormError } from '#lib/formErrors.js';

	const emailId = 'style-guide-error-summary-email';
	const passwordId = 'style-guide-error-summary-password';

	let email = $state('');
	let password = $state('');
	let errors = $state<FormError[]>([]);

	function errorFor(targetId: string): string | undefined {
		return errors.find((entry) => entry.targetId === targetId)?.message;
	}

	function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errors = [];
		const found: FormError[] = [];
		if (email.trim() === '') found.push({ message: 'Enter your email address', targetId: emailId });
		if (password === '') found.push({ message: 'Enter your password', targetId: passwordId });
		errors = found;
	}
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Error summary" />
	<Text
		text="GOV.UK's error summary. When a submit is refused, the page says so once, at the top, and lists every reason with a link to the control that caused it. It takes focus the moment it appears, so the refusal is announced and the first fix is one Tab away."
		tone="variant"
	/>
	<Text
		text="It is not a Notice. A Notice announces an outcome — an invoice sent, a Contract saved. This announces that a submission was refused, which is why the two never share a page region."
		tone="variant"
	/>

	<Heading level={3} variant="card" text="One reason, and several" />
	<stack-l space="var(--space-6)">
		<ErrorSummary errors={[{ message: 'Enter your email address', targetId: emailId }]} />
		<ErrorSummary
			errors={[
				{ message: 'Enter the name of your Practice', targetId: emailId },
				{ message: 'Choose the state you work from', targetId: emailId },
				{ message: 'Password must be 6 characters or more', targetId: passwordId }
			]}
		/>
	</stack-l>

	<Heading level={3} variant="card" text="A refusal that belongs to no field" />
	<Text
		text="The service was unreachable, or the server refused for a reason nothing on the page caused. The entry is plain text, because a link that goes nowhere useful is worse than no link."
		tone="variant"
	/>
	<ErrorSummary
		errors={[{ message: 'There is a problem with the service. Try again in a few minutes.' }]}
	/>

	<Heading level={3} variant="card" text="On a real form" />
	<Text
		text="Submit this empty to see what a refused form does: the summary appears and takes focus, each entry links to its field, and the message beside the field is word-for-word the entry above."
		tone="variant"
	/>
	<form onsubmit={handleSubmit} novalidate>
		<stack-l space="var(--space-5)">
			<ErrorSummary {errors} />
			<LabeledField id={emailId} label="Email" error={errorFor(emailId)}>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="email"
						value={email}
						onInput={(value) => (email = value)}
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
					/>
				{/snippet}
			</LabeledField>
			<Button type="submit" label="Continue" />
		</stack-l>
	</form>
</stack-l>

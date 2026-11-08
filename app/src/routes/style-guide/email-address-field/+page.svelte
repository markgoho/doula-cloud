<script lang="ts">
	import EmailAddressField from '#lib/components/molecules/EmailAddressField.svelte';

	let email = $state('');
</script>

<stack-l space="var(--space-6)">
	<h1>Email address field</h1>

	<p>
		The email box every sign-in, reset and recovery screen asks with. It names itself as the
		username so a password manager files the login under it.
	</p>

	<EmailAddressField id="style-guide-email" value={email} onInput={(value) => (email = value)} />

	<EmailAddressField
		id="style-guide-email-refused"
		value={email}
		onInput={(value) => (email = value)}
		error="Enter your email address"
	/>
</stack-l>

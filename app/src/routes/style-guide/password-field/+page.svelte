<script lang="ts">
	import PasswordField from '#lib/components/molecules/PasswordField.svelte';

	let password = $state('');
</script>

<stack-l space="var(--space-6)">
	<h1>Password field</h1>

	<p>
		The password box for proving who you are to an account you already have. It tells the browser
		it holds the current password, so a password manager offers to fill it.
	</p>

	<PasswordField id="style-guide-password" value={password} onInput={(value) => (password = value)} />

	<PasswordField
		id="style-guide-password-refused"
		value={password}
		onInput={(value) => (password = value)}
		error="Enter your password"
	/>
</stack-l>

<script lang="ts">
	import TextInput from '#lib/components/atoms/TextInput.svelte';

	const noop = () => {};

	let name = $state('');
	let email = $state('');
	let quantity = $state('1');
	let dueDate = $state('');
</script>

<stack-l space="var(--space-6)">
	<h1>Text input</h1>

	<section>
		<h2>Default</h2>
		<TextInput value={name} onInput={(value) => (name = value)} placeholder="Full name" />
	</section>

	<section>
		<h2>Email type</h2>
		<TextInput
			type="email"
			value={email}
			onInput={(value) => (email = value)}
			placeholder="you@example.com"
		/>
	</section>

	<section>
		<h2>Number type</h2>
		<TextInput
			type="number"
			value={quantity}
			onInput={(value) => (quantity = value)}
			placeholder="1"
		/>
	</section>

	<section>
		<h2>Date type</h2>
		<TextInput type="date" value={dueDate} onInput={(value) => (dueDate = value)} />
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any control on this page to see its focus outline.</p>
	</section>

	<section>
		<h2>Invalid</h2>
		<TextInput value="" onInput={noop} invalid />
	</section>

	<section>
		<h2>Disabled</h2>
		<TextInput value="Read-only value" onInput={noop} disabled />
	</section>
</stack-l>

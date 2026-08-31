<script lang="ts">
	import TextInput from '#lib/components/atoms/TextInput.svelte';

	const noop = () => {};

	let name = $state('');
	let email = $state('');
	let quantity = $state('1');
	let dueDate = $state('');
	let password = $state('correct horse battery staple');
</script>

<stack-l space="var(--space-6)">
	<h1>Text input</h1>

	<section>
		<h2>Default</h2>
		<!--
			The longest realistic value, not a representative one (ADR-0025): a
			full legal name with a hyphenated double-barrelled surname, which is
			what a Client types into this field, and the longest email a Practice
			hands out. Neither overflows on its own -- a browser breaks on "-"
			and "@" (#521) -- so what they test is the field's own width, not the
			row's.
		-->
		<TextInput
			value={name}
			onInput={(value) => (name = value)}
			placeholder="Full legal name, as it appears on the Contract"
		/>
	</section>

	<section>
		<h2>Email type</h2>
		<TextInput
			type="email"
			value={email}
			onInput={(value) => (email = value)}
			placeholder="anne-marie.ochieng-whitfield@highland-midwifery-group.example.org"
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

	<!--
		The toggle owns a button and a state, which is more than TextInput
		does for any other type -- it lives here rather than as a new atom,
		the same call #404 made for type="date" (#470).
	-->
	<section>
		<h2>Password type</h2>
		<p>Defaults to hidden. The toggle reveals the value; paste still works.</p>
		<TextInput type="password" value={password} onInput={(value) => (password = value)} />
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
		<TextInput value="Anne-Marie Ochieng-Whitfield" onInput={noop} disabled />
	</section>
</stack-l>

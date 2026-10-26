<script lang="ts">
	import TextInput, { type InputWidth } from '#lib/components/atoms/TextInput.svelte';

	const noop = () => {};
	const widths: readonly InputWidth[] = [2, 3, 4, 5, 10, 20, 30];
	let fee = $state('2500.00');
	let days = $state('365');

	let name = $state('Anne-Marie Ochieng-Whitfield');
	let email = $state('anne-marie.ochieng-whitfield@highland-midwifery-group.example.org');
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
			full legal name with a hyphenated double-barreled surname, which is
			what a Client types into this field, and the longest email a Practice
			hands out. Neither overflows on its own -- a browser breaks on "-"
			and "@" (#521) -- so what they test is the field's own width, not the
			row's. Both are values, not placeholders: this atom has no
			placeholder, because GOV.UK's Text input guidance refuses one
			(#1751). A hint goes on LabeledField.
		-->
		<TextInput value={name} onInput={(value) => (name = value)} />
	</section>

	<section>
		<h2>Email type</h2>
		<TextInput type="email" value={email} onInput={(value) => (email = value)} />
	</section>

	<section>
		<h2>Number type</h2>
		<TextInput type="number" value={quantity} onInput={(value) => (quantity = value)} />
	</section>

	<section>
		<h2>Date type</h2>
		<p>Sized to its own segments, not to the column.</p>
		<TextInput type="date" value={dueDate} onInput={(value) => (dueDate = value)} />
	</section>

	<!--
		The scale (#1216): each step filled with as many wide digits as it
		claims to hold, so a step that clipped or widened shows here first.
	-->
	<section>
		<h2>Widths</h2>
		<p>GOV.UK's steps, in characters. Each box holds its count; the last one shrinks with a narrow column.</p>
		<stack-l space="var(--space-3)">
			{#each widths as width (width)}
				<TextInput ariaLabel={`Width ${width}`} {width} value={'8'.repeat(width)} onInput={noop} />
			{/each}
		</stack-l>
	</section>

	<section>
		<h2>Prefix and suffix</h2>
		<p>The unit says what the narrow box holds. The label says it again for a screen reader.</p>
		<stack-l space="var(--space-3)">
			<TextInput ariaLabel="Fee (USD)" type="number" width={10} prefix="$" value={fee} onInput={(value) => (fee = value)} />
			<TextInput ariaLabel="Days to pay" type="number" width={3} suffix="days" value={days} onInput={(value) => (days = value)} />
		</stack-l>
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

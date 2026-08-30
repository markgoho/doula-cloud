<script lang="ts">
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	const noop = () => {};

	let note = $state('');
	let prose = $state('');
	let fact = $state('');
	let labelled = $state('');
</script>

<stack-l space="var(--space-6)">
	<h1>Textarea</h1>

	<section>
		<h2>Default</h2>
		<p>Four rows to start with. Type past them and the field grows, up to twenty rows.</p>
		<Textarea value={note} onInput={(value) => (note = value)} placeholder="Anything you want to add" />
	</section>

	<section>
		<h2>A taller starting height</h2>
		<p>What a contract body asks for: <code>rows</code> is a starting height, not a cap.</p>
		<Textarea value={prose} onInput={(value) => (prose = value)} rows={12} />
	</section>

	<section>
		<h2>Character count</h2>
		<p>
			Only where the server enforces a maximum. Type past 40 and the count goes negative and turns
			red; nothing is truncated, because a hard cap eats pasted text silently.
		</p>
		<Textarea value={fact} onInput={(value) => (fact = value)} maxLength={40} />
	</section>

	<section>
		<h2>Inside a labeled field</h2>
		<LabeledField
			label="What your Practice offers"
			hint="Say what kind of support you provide and where you work."
			error={labelled.length > 0 ? '' : 'Enter a description of what your Practice offers'}
		>
			{#snippet children({ id, describedBy, invalid })}
				<Textarea {id} {describedBy} {invalid} value={labelled} onInput={(value) => (labelled = value)} />
			{/snippet}
		</LabeledField>
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any control on this page to see its focus outline.</p>
	</section>

	<section>
		<h2>Invalid</h2>
		<Textarea value="" onInput={noop} invalid />
	</section>

	<section>
		<h2>Disabled</h2>
		<Textarea value="Read-only value" onInput={noop} disabled />
	</section>
</stack-l>

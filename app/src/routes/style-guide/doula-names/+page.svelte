<script lang="ts">
	import DoulaNames from '#lib/components/molecules/DoulaNames.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	const THREE = ['Joss Adeyemi', 'Lena Vasquez', 'Priya Raman'];
	const IDS = ['a', 'b', 'c'];
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Doula names" />
	<Text
		text="A row's Doulas on a Staff list. The Contracts list and the Clients list draw the Doula cell with it: &quot;You&quot; first where the reader is one of them, every name where they fit a one-line cell, else the first name and a count. A screen reader hears every name."
		tone="variant"
	/>

	<stack-l space="var(--space-4)">
		<h2>In a table: one line</h2>
		<stack-l space="var(--space-2)">
			<div><DoulaNames names={['Joss Adeyemi']} ids={['a']} isTable /></div>
			<div><DoulaNames names={['Jo Ames', 'Li Cho']} ids={['a', 'b']} isTable /></div>
			<div><DoulaNames names={THREE} ids={IDS} isTable /></div>
			<div><DoulaNames names={THREE} ids={IDS} readerId="b" isTable /></div>
		</stack-l>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>On a card: every name, wrapping</h2>
		<div><DoulaNames names={THREE} ids={IDS} readerId="c" /></div>
	</stack-l>
</stack-l>

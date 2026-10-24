<!--
	Test support, not a component anybody ships.

	A sentence that ends with a `Link`, and a button the sentence describes
	(#1808). A spec cannot write the full stop as a text node after the
	`Link` without a component around both, and the button's accessible
	description is where a screen reader hears the sentence -- the same
	shape as the agreement sentence on `/signup`.
-->
<script lang="ts">
	import Button from '../Button.svelte';
	import Link from '../Link.svelte';

	let { href, label }: { href: string; label: string } = $props();

	const id = $props.id();
</script>

<p {id}>Read the <Link {href} {label} />.</p>
<Button label="Continue" describedBy={id} />

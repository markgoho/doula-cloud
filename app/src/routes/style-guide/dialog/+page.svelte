<script lang="ts">
	import Button from '#lib/components/atoms/Button.svelte';
	import Dialog from '#lib/components/molecules/Dialog.svelte';

	let isOpen = $state(false);
</script>

<stack-l space="var(--space-6)">
	<h1>Dialog</h1>

	<section>
		<h2>Default</h2>
		<p>
			Wraps the native <code>&lt;dialog&gt;</code>, opened with <code>showModal()</code> -- the top
			layer, light dismiss, Escape and focus return are the browser's job, never hand-rolled.
		</p>
		<Button label="Open the Contract you are about to sign" onClick={() => (isOpen = true)} />
		<Dialog bind:open={isOpen} label="Contract with Highland Midwifery &amp; Birth Support Collective">
			<p>
				The dialog's content, handed to it as a snippet -- here a whole Contract
				paragraph, because that is what the app actually puts in one:
				Anne-Marie Ochieng-Whitfield agrees to pay $4,250.00 to Highland
				Midwifery &amp; Birth Support Collective of Western New York.
			</p>
			<Button label="Close" variant="secondary" onClick={() => (isOpen = false)} />
		</Dialog>
	</section>
</stack-l>

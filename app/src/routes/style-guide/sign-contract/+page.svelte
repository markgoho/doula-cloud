<script lang="ts">
	import SignContract from '#lib/components/organisms/SignContract.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Sign contract</h1>

	<section>
		<h2>The electronic-signature disclosure comes first</h2>
		<SignContract onSign={async () => {}} />
	</section>
</stack-l>

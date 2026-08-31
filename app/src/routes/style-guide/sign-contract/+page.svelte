<script lang="ts">
	import SignContract from '#lib/components/organisms/SignContract.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Sign contract</h1>

	<section>
		<h2>The electronic-signature disclosure comes first</h2>
		<!--
			No hostile value exists for this component (ADR-0025): it takes no
			Practice content at all -- the disclosure and the consent wording are
			the product's own, and both are rendered here in full.
		-->
		<SignContract onSign={async () => {}} />
	</section>
</stack-l>

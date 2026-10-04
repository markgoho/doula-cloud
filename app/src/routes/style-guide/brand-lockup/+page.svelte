<script lang="ts">
	import BrandLockup from '#lib/components/molecules/BrandLockup.svelte';
	import Link from '#lib/components/atoms/Link.svelte';

	const sizes = ['sm', 'md', 'lg'] as const;
</script>

<stack-l space="var(--space-6)">
	<h1>Brand lockup</h1>

	<section>
		<h2>The product's name, drawn once</h2>
		<p>
			Every bar that says who we are renders this rather than its own mark-and-words pair. The name
			is one word, <code>DoulaCloud</code>, capital D and capital C, as
			<Link href="https://github.com/markgoho/doula-cloud/issues/338" label="#338" /> settled.
		</p>
		<!--
			No hostile value exists for this component (ADR-0025): the lockup
			takes no Practice content at all, and its one string is the
			product's own name, which never gets longer.
		-->
		<stack-l space="var(--space-6)">
			{#each sizes as size (size)}
				<BrandLockup {size} />
			{/each}
		</stack-l>
	</section>
</stack-l>

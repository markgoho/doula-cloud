<script lang="ts">
	import BrandLockup from '#lib/components/molecules/BrandLockup.svelte';

	/*
	 * The reduced bar every signed-out screen carries (#431): the lockup
	 * alone, at the same 60px height and behind the same hairline as the
	 * real shell. That is the whole point of it -- signing in fills the bar
	 * in rather than making one appear, so nothing below it reflows at the
	 * moment a person's session starts.
	 *
	 * It takes no props. The Client portal's signed-out screens use it too,
	 * and say `Doula Cloud` rather than the Practice's name: /portal/login
	 * has no session, so the app does not yet know which Practice the person
	 * belongs to. That is accepted, not a defect to design around.
	 */
</script>

<header>
	<BrandLockup />
</header>

<style>
	@layer components {
		header {
			display: flex;
			align-items: center;
			block-size: var(--top-bar-height);
			padding-inline: var(--page-gutter);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-bright);
		}
	}
</style>

<script lang="ts">
	import MenuButton from '#lib/components/molecules/MenuButton.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Menu button</h1>

	<section>
		<h2>A control that reveals a panel beside itself</h2>
		<p>
			Native <code>popover</code> does the work: the browser puts the panel in the top layer,
			dismisses it on a click outside or on Escape, and returns focus to the trigger. The only
			script in the component mirrors the open state onto <code>aria-expanded</code>.
		</p>
		<p>
			Deliberately not <code>role="menu"</code>. That role promises arrow-key navigation between
			items, and these panels hold ordinary links and buttons that Tab already reaches.
		</p>
		<cluster-l space="var(--space-6)" align="center">
			<MenuButton label="Aligned to the inline end" icon="caret-down" align="end">
				<p class="item">The panel's inline end lines up with the trigger's.</p>
			</MenuButton>
			<MenuButton label="Aligned to the inline start" icon="caret-down" align="start">
				<p class="item">The panel's inline start lines up with the trigger's.</p>
			</MenuButton>
		</cluster-l>
	</section>
</stack-l>

<style>
	.item {
		margin: 0;
		padding: var(--space-3) var(--space-4);
		font-size: var(--text-body-sm-size);
	}
</style>

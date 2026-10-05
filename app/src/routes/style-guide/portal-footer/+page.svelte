<script lang="ts">
	import PortalFooter from '#lib/components/organisms/PortalFooter.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Portal footer</h1>

	<p>
		The foot of every Portal screen, signed in or not: a link to the Privacy Policy that opens in a
		new tab and says so. It is a notice, so nothing in it asks for agreement.
	</p>

	<!--
		No hostile value exists for this component (ADR-0025): the footer
		holds one fixed link, so no Practice content can reach it.
	-->
	<PortalFooter />
</stack-l>

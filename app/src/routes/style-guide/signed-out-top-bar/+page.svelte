<script lang="ts">
	import SignedOutTopBar from '#lib/components/organisms/SignedOutTopBar.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Signed-out top bar</h1>

	<p>
		The lockup alone, at the same 60px height and behind the same hairline as the real shell. That
		is the whole point of it: signing in fills the bar in rather than making one appear, so nothing
		below it reflows at the moment a person's session starts.
	</p>

	<!--
		No hostile value exists for this component (ADR-0025): the bar holds
		the lockup and nothing else, so no Practice content can reach it.
	-->
	<SignedOutTopBar />
</stack-l>

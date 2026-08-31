<script lang="ts">
	import Text from '#lib/components/atoms/Text.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Text</h1>

	<section>
		<h2>Type steps</h2>
		<p>
			Four of the brief's eight steps. A call site names the purpose, never a size — the step
			names are the brief's own, and the sizes behind them live only in tokens.css.
		</p>
		<stack-l space="var(--space-4)">
			<Text step="body" text="Body — prose, form values and list rows. Anne-Marie Ochieng-Whitfield asked for a home birth supported by Highland Midwifery &amp; Birth Support Collective." />
			<Text step="body-sm" text="Body small — table cells and dense rows, such as anne-marie.ochieng-whitfield@highland-midwifery-group.example.org." />
			<Text step="label" text="Label — field labels, nav items and buttons, such as I have read this Contract and I am signing it electronically." />
			<Text step="meta" text="Meta — timestamps, attribution and help text, such as Sent by Anne-Marie Ochieng-Whitfield on 14 September 2027 at 09:42." />
		</stack-l>
	</section>

	<section>
		<h2>Color tones</h2>
		<p>
			Tone is a separate axis from step, because how big a thing is and how much it matters do
			not always move together. Every tone clears 4.5:1 on every surface, in both themes.
		</p>
		<stack-l space="var(--space-4)">
			<Text tone="default" text="Default — body and primary reading copy." />
			<Text tone="variant" text="Variant — secondary copy." />
			<Text tone="muted" text="Muted — metadata, the quietest readable tone." />
		</stack-l>
	</section>

	<section>
		<h2>Combined</h2>
		<stack-l space="var(--space-4)">
			<Text step="meta" tone="muted" text="Sent by Anne-Marie Ochieng-Whitfield · 14 September 2027, 09:42" />
		</stack-l>
	</section>
</stack-l>

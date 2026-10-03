<script lang="ts">
	import WarningText from '#lib/components/atoms/WarningText.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Warning text</h1>

	<section>
		<h2>Default</h2>
		<WarningText message="This removes the Doula from the Practice, ending every Engagement the Doula is working on. This cannot be undone." />
	</section>

	<!-- A message long enough to wrap at any width the sweep gives it, so
	     the icon beside it is measured under the most pressure (#1739). -->
	<section>
		<h2>Long message</h2>
		<WarningText message="This permanently destroys Maria Esperanza Delacroix-Whitfield's name, email, phone, address and date of birth. Every Engagement, Contract, Invoice and Visit for Maria Esperanza Delacroix-Whitfield stays in the Practice's financial and clinical record. Stripe keeps each payment record for 90 days after the invoice was paid. Messages, signed Contract wording and Plan Instance answers keep the words written in them. This cannot be undone." />
	</section>
</stack-l>

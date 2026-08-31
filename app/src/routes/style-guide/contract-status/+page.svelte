<script lang="ts">
	import ContractStatus from '#lib/components/molecules/ContractStatus.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Contract status</h1>

	<!--
		No hostile value exists for this component (ADR-0025): it takes a
		status and nothing else, so its longest realistic value is the longest
		of the three states -- and all three are rendered below.
	-->

	<section>
		<h2>Draft</h2>
		<ContractStatus status="draft" />
	</section>

	<section>
		<h2>Signed, seen by Staff &mdash; Void is offered</h2>
		<ContractStatus status="signed" onVoid={async () => {}} />
	</section>

	<section>
		<h2>Signed, seen by the Client &mdash; no onVoid, so no button</h2>
		<ContractStatus status="signed" />
	</section>

	<section>
		<h2>Voided, the terminal state</h2>
		<ContractStatus status="voided" onVoid={async () => {}} />
	</section>
</stack-l>

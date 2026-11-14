<script lang="ts">
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ShowOnly from '#lib/components/molecules/ShowOnly.svelte';

	let isMine = $state(false);
	let isNeedingCover = $state(false);
	let isOwing = $state(false);
	let isNoContract = $state(false);
</script>

<stack-l space="var(--space-6)">
	<h1>Show only</h1>

	<section>
		<h2>Default</h2>
		<ShowOnly>
			<LabeledField label="Mine" orientation="inline">
				{#snippet children(control)}
					<Checkbox name="mine" value="true" checked={isMine} onChange={(checked) => (isMine = checked)} {...control} />
				{/snippet}
			</LabeledField>
			<LabeledField label="Needing cover" orientation="inline">
				{#snippet children(control)}
					<Checkbox name="cover" value="needed" checked={isNeedingCover} onChange={(checked) => (isNeedingCover = checked)} {...control} />
				{/snippet}
			</LabeledField>
		</ShowOnly>
	</section>

	<!--
		The longest labels a bar carries (ADR-0025): the Clients list's three
		boxes wrap among themselves rather than past the edge.
	-->
	<section>
		<h2>Long labels</h2>
		<ShowOnly>
			<LabeledField label="Who owe money" orientation="inline">
				{#snippet children(control)}
					<Checkbox name="owes" value="true" checked={isOwing} onChange={(checked) => (isOwing = checked)} {...control} />
				{/snippet}
			</LabeledField>
			<LabeledField label="Active, with no Contract" orientation="inline">
				{#snippet children(control)}
					<Checkbox name="noContract" value="true" checked={isNoContract} onChange={(checked) => (isNoContract = checked)} {...control} />
				{/snippet}
			</LabeledField>
			<LabeledField label="Doula no longer on this work" orientation="inline">
				{#snippet children(control)}
					<Checkbox name="needsDoula" value="true" checked={isMine} onChange={(checked) => (isMine = checked)} {...control} />
				{/snippet}
			</LabeledField>
		</ShowOnly>
	</section>
</stack-l>

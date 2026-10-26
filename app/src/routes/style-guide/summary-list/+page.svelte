<script lang="ts">
	import Link from '#lib/components/atoms/Link.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import SummaryList, { type SummaryListRow } from '#lib/components/molecules/SummaryList.svelte';

	/*
	 * The longest realistic values beside short ones (ADR-0025): a
	 * Practice's own names for two Doulas, and a pasted URL with no break
	 * opportunity, so the drag surface shows how a row wraps. The
	 * bordered list draws one row open, the way a pressed Change leaves
	 * it, so the continuum check measures the in-place form too; this
	 * page has no control of its own (#1638).
	 */
	const rows: SummaryListRow[] = [
		{ label: 'Care', value: 'Birth', changes: 'care' },
		{ label: 'Due', value: 'Nov 14, 2026' },
		{ label: 'Care phase', value: 'Active', changes: 'care phase', isDetailShown: true },
		{ label: 'Doula', value: 'Anne-Marie Ochieng-Whitfield, Priya Raman', changes: 'Doula' },
		{
			label: 'Referral',
			value: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake'
		}
	];
</script>

<stack-l space="var(--space-6)">
	<h1>Summary list</h1>

	<section>
		<h2>Bordered, with one in-place Change open</h2>
		<SummaryList {rows} isBordered>
			{#snippet action(row, describedBy)}
				<Button label="Change" variant="link" expanded={row.isDetailShown ?? false} {describedBy} />
			{/snippet}
			{#snippet detail()}
				<Button label="Mark care complete" size="sm" variant="secondary" />
			{/snippet}
		</SummaryList>
	</section>

	<section>
		<h2>Plain, with a Change link</h2>
		<SummaryList {rows}>
			{#snippet action(row, describedBy)}
				<Link href="#{row.label}" label="Change" variant="secondary" {describedBy} />
			{/snippet}
		</SummaryList>
	</section>
</stack-l>

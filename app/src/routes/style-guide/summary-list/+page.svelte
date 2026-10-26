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

	/*
	 * A record's facts, with no action: the shape most lists have. The
	 * label is free text too (#725) -- a Client Field Template's label is
	 * a question the Practice types -- so the same URL sits on the key
	 * side of a row as well, and the Contract row's value is a link.
	 */
	const facts: SummaryListRow[] = [
		{ label: 'Status', value: 'Stripe onboarding incomplete' },
		{ label: 'Contract', value: 'Signed Sep 14, 2027' },
		{ label: 'Address', value: '128 Meadowbrook Lane, Apartment 4B, Rochester, NY 14620' },
		{
			label: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			value: 'Epidural, as early as it can be given'
		}
	];

	// A Birth Plan section: each question is a sentence a Practice wrote.
	const answers: SummaryListRow[] = [
		{ label: 'Who do you want in the room with you?', value: 'My partner, Sam, and my sister, Leila' },
		{ label: 'Would you like photos taken during the birth?', value: 'Yes' },
		{ label: 'Anything else your doula should know?', value: '—' }
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
		<h2>A record's facts, one value a link</h2>
		<SummaryList rows={facts}>
			{#snippet value(row)}
				{#if row.label === 'Contract'}
					<Link href="#contract" label={row.value} />
				{:else}
					{row.value}
				{/if}
			{/snippet}
		</SummaryList>
	</section>

	<section>
		<h2>Stacked, a question above its answer</h2>
		<SummaryList rows={answers} isStacked />
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

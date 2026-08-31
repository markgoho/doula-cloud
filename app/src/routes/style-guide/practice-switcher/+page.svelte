<script lang="ts">
	import PracticeSwitcher, {
		type PracticeOption
	} from '#lib/components/molecules/PracticeSwitcher.svelte';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): a
	 * Practice registers under its legal name, which is longer than the one
	 * it puts on a sign, and this control has to show all of it.
	 */
	const several: PracticeOption[] = [
		{
			practiceId: 'p1',
			practiceName: 'Highland Midwifery & Birth Support Collective of Western New York',
			roles: ['owner', 'admin'],
			href: '#riverside'
		},
		{
			practiceId: 'p2',
			practiceName: 'Finger Lakes Birth Support and Postpartum Care Cooperative',
			roles: ['doula'],
			href: '#finger-lakes'
		},
		{
			practiceId: 'p3',
			practiceName: 'Genesee Valley Midwifery and Lactation Consultants',
			roles: ['doula'],
			href: '#genesee'
		}
	];
</script>

<stack-l space="var(--space-6)">
	<h1>Practice switcher</h1>

	<section>
		<h2>Several Memberships</h2>
		<p>
			A contractor Doula holds Memberships at several agencies — the pilot has one working at three
			— so this is not an edge case bolted onto a single-Practice product.
		</p>
		<PracticeSwitcher practices={several} currentPracticeId="p1" />
	</section>

	<section>
		<h2>One Membership</h2>
		<p>
			Still named, because a person should be able to see which Practice she is in. No caret and no
			panel: a control that opens a list of one is a promise the product cannot keep.
		</p>
		<PracticeSwitcher practices={[several[0]!]} currentPracticeId="p1" />
	</section>
</stack-l>

<script lang="ts">
	import PracticeSwitcher, {
		type PracticeOption
	} from '#lib/components/molecules/PracticeSwitcher.svelte';

	const several: PracticeOption[] = [
		{
			practiceId: 'p1',
			practiceName: 'Riverside Doula Collective',
			roles: ['owner', 'admin'],
			href: '#riverside'
		},
		{
			practiceId: 'p2',
			practiceName: 'Finger Lakes Birth Support',
			roles: ['doula'],
			href: '#finger-lakes'
		},
		{
			practiceId: 'p3',
			practiceName: 'Genesee Valley Midwifery',
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

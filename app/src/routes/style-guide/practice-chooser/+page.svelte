<script lang="ts">
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import PracticeChooser from '#lib/components/molecules/PracticeChooser.svelte';
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Practice chooser" />
	<Text
		text="Where a Staff member with more than one Practice picks which to open: on /, and on /login once the password is accepted. A door per Practice, the name as the link and the roles held there as the line under it."
		tone="variant"
	/>

	<!--
		Two rows (ADR-0025's Fixtures section): the longest name a Practice
		has registered (#530's URL) with every role, beside a short name with
		one. A Membership always holds at least one role, so there is no
		empty line to show.
	-->
	<PracticeChooser
		practices={[
			{
				practiceId: 'practice-1',
				practiceName: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
				roles: ['owner', 'admin', 'doula'],
				employmentType: 'employee',
				href: '/style-guide/practice-chooser'
			},
			{
				practiceId: 'practice-2',
				practiceName: 'Hilltop Doulas',
				roles: ['doula'],
				employmentType: 'contractor',
				href: '/style-guide/practice-chooser'
			}
		]}
	/>
</stack-l>

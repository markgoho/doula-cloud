<script lang="ts">
	import Avatar, { initialsOf } from '#lib/components/atoms/Avatar.svelte';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): a
	 * hyphenated double-barreled surname beside a three-word name, because
	 * the name renders next to the circle here and it is the name, not the
	 * circle, that decides how wide this row gets.
	 */
	const names = [
		{ firstName: 'Anne-Marie', lastName: 'Ochieng-Whitfield' },
		{ firstName: 'Renata Chiamaka', lastName: 'Okonkwo-Adeyemi' },
		{ firstName: 'Prince', lastName: '' },
		{ firstName: 'dee', lastName: 'marchetti' }
	];
</script>

<stack-l space="var(--space-6)">
	<h1>Avatar</h1>

	<section>
		<h2>Initials, taken from the two name fields</h2>
		<p>
			The initials are worked out here rather than served: both names are already on the wire, and a
			second field holding two letters of them is a copy that can go stale. One letter from the
			first name and one from the last, so a name of several words never splits on a space and never
			adds a third letter to a 34px circle.
		</p>
		<cluster-l space="var(--space-5)" align="center">
			{#each names as { firstName, lastName } (`${firstName} ${lastName}`)}
				<cluster-l space="var(--space-2)" align="center">
					<Avatar {firstName} {lastName} />
					<span>{firstName} {lastName} &rarr; {initialsOf(firstName, lastName)}</span>
				</cluster-l>
			{/each}
		</cluster-l>
	</section>

	<section>
		<h2>It never carries the identity on its own</h2>
		<p>
			The circle is <code>aria-hidden</code> in every case. It always sits inside a control that
			names the person in real text, so announcing two initials as well would only repeat a worse
			version of the same fact.
		</p>
	</section>
</stack-l>

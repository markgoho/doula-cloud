<script lang="ts">
	import Avatar, { initialsOf } from '#lib/components/atoms/Avatar.svelte';

	const names = ['Mark Goho', 'Tasha Bell', 'Renata Okonkwo Adeyemi', 'Prince', 'dee marchetti'];
</script>

<stack-l space="var(--space-6)">
	<h1>Avatar</h1>

	<section>
		<h2>Initials, derived from the name</h2>
		<p>
			The initials are worked out here rather than served: the name is already on the wire, and a
			second field holding two letters of it is a copy that can go stale. First and last word only,
			so a middle name adds no third letter to a 34px circle.
		</p>
		<cluster-l space="var(--space-5)" align="center">
			{#each names as name (name)}
				<cluster-l space="var(--space-2)" align="center">
					<Avatar {name} />
					<span>{name} &rarr; {initialsOf(name)}</span>
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

<script lang="ts">
	import AvatarMenu from '#lib/components/molecules/AvatarMenu.svelte';
	import type { SignOutOutcome } from '#lib/signOut.js';

	function signOut(): Promise<SignOutOutcome> {
		return Promise.resolve({ ok: true });
	}

	function refuses(): Promise<SignOutOutcome> {
		return Promise.resolve({ ok: false, message: 'We could not sign you out. Try again.' });
	}
</script>

<stack-l space="var(--space-6)">
	<h1>Avatar menu</h1>

	<section>
		<h2>The person, never the Practice</h2>
		<p>
			Practice-scoped settings are a nav item. What lives here is who is signed in, and the two
			things she can do about that.
		</p>
		<!--
			The longest realistic value, not a representative one (ADR-0025): the
			name and the address are what decide how wide the open panel gets,
			and a Practice hands out addresses on its own domain.
		-->
		<AvatarMenu
			name="Anne-Marie Ochieng-Whitfield"
			email="anne-marie.ochieng-whitfield@highland-midwifery-group.example.org"
			accountHref="#account"
			{signOut}
		/>
	</section>

	<section>
		<h2>The Client portal's</h2>
		<p>
			No email and no Account link: the portal has no per-person screen to reach, and the Client's
			address is not something the portal asks her to check.
		</p>
		<AvatarMenu name="Renata Chiamaka Okonkwo-Adeyemi" signOut={refuses} />
	</section>

	<section>
		<h2>Before the session lands</h2>
		<p>
			With no name yet there is nobody to name, so the component holds the 44px and nothing else.
			That keeps the bar's inline end still rather than having the avatar shove the row when the
			session arrives.
		</p>
		<AvatarMenu name="" {signOut} />
	</section>
</stack-l>

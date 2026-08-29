<script lang="ts">
	import Avatar from '#lib/components/atoms/Avatar.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import MenuButton from './MenuButton.svelte';
	import SignOutButton from './SignOutButton.svelte';
	import type { SignOutOutcome } from '#lib/signOut.js';

	/*
	 * The person, never the Practice (#431). Practice-scoped settings are a
	 * nav item; what lives here is who is signed in and the two things she
	 * can do about that.
	 *
	 * `accountHref` is optional because the Client portal has no per-person
	 * screen to link to. On the Staff side it is not optional in practice:
	 * /account is where a Doula corrects her own work state (#437), and this
	 * menu is the only chrome that can reach it now the temporary header of
	 * links is gone. The drawing showed identity and Sign out alone; adding
	 * the link is a deliberate departure recorded on the ticket, and it is
	 * the same principle -- a work state is a fact about her, not about any
	 * Practice she works at.
	 */
	interface Properties {
		name: string;
		email?: string;
		accountHref?: string;
		signOut: () => Promise<SignOutOutcome>;
	}

	let { name, email, accountHref, signOut }: Properties = $props();
</script>

{#snippet face()}
	<Avatar {name} />
{/snippet}

<!--
	The name arrives from a request, so for one paint there is nobody to
	name. An empty placeholder of the same 44px keeps the bar's inline end
	still rather than having the avatar shove the row when the session
	lands -- the brief's smoothness requirement, applied to the one part of
	the chrome that is not known at first paint.
-->
{#if name === ''}
	<span class="placeholder" aria-hidden="true"></span>
{:else}
	<MenuButton label="Your account, {name}" iconOnly visual={face} align="end">
		<div class="identity">
			<p class="name">{name}</p>
			{#if email}
				<p class="email">{email}</p>
			{/if}
		</div>
		<div class="items">
			{#if accountHref}
				<Link href={accountHref} label="Account" variant="secondary" />
			{/if}
			<SignOutButton {signOut} />
		</div>
	</MenuButton>
{/if}

<style>
	@layer components {
		.placeholder {
			display: inline-block;
			inline-size: var(--hit-target-min);
			block-size: var(--hit-target-min);
		}

		.identity {
			display: flex;
			flex-direction: column;
			gap: var(--space-1);
			padding: var(--space-3) var(--space-4);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
		}

		.name {
			margin: 0;
			font-size: var(--text-body-sm-size);
			font-weight: var(--font-weight-semibold);
		}

		.email {
			margin: 0;
			overflow-wrap: anywhere;
			color: var(--color-on-surface-muted);
			font-size: var(--text-meta-size);
		}

		.items {
			display: flex;
			flex-direction: column;
			align-items: flex-start;
			gap: var(--space-2);
			padding: var(--space-3) var(--space-4);
		}
	}
</style>

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
		firstName: string;
		lastName?: string;
		email?: string;
		accountHref?: string;
		signOut: () => Promise<SignOutOutcome>;
	}

	let { firstName, lastName = '', email, accountHref, signOut }: Properties = $props();

	// First then last, the order every screen prints a Staff member's name in (#1537).
	function fullName(): string {
		return lastName ? `${firstName} ${lastName}` : firstName;
	}

	// Where an address too wide for the panel may wrap: after the @ and
	// before each dot of the domain, never mid-label (#2226).
	function emailParts(address: string): string[] {
		const at = address.lastIndexOf('@');
		const [first, ...rest] = address.slice(at + 1).split('.');
		return [address.slice(0, at + 1), first, ...rest.map((label) => `.${label}`)];
	}
</script>

{#snippet face()}
	<Avatar {firstName} {lastName} />
{/snippet}

<!--
	The name arrives from a request, so for one paint there is nobody to
	name. An empty placeholder of the same 44px keeps the bar's inline end
	still rather than having the avatar shove the row when the session
	lands -- the brief's smoothness requirement, applied to the one part of
	the chrome that is not known at first paint.
-->
{#if firstName === ''}
	<span class="placeholder" aria-hidden="true"></span>
{:else}
	<MenuButton label="Your account, {fullName()}" iconOnly visual={face} align="end">
		<div class="identity">
			<p class="name">{fullName()}</p>
			{#if email}
				<p class="email">
					{#each emailParts(email) as part, index (index)}{#if index > 0}<wbr />{/if}{part}{/each}
				</p>
			{/if}
		</div>
		<!--
			Rows, not a link and a button card (#2141): each item is the
			panel's full width at one weight. Sign out sits apart from
			Account under a divider, because it ends the session where
			Account goes somewhere. In the panel's own color, not the
			drawing's red: signing out loses nothing, and red is the brief's
			signal for an error or a destructive act; GOV.UK's header draws
			Sign out as a plain link. With no Account (the Client portal) the
			identity's own rule is the divider.
		-->
		<div class="items">
			{#if accountHref}
				<Link href={accountHref} label="Account" variant="menu" />
			{/if}
			<div class="sign-out" class:divided={Boolean(accountHref)}>
				<SignOutButton {signOut} variant="menu" />
			</div>
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
			padding-block: var(--space-1) 0;
		}

		.sign-out {
			display: flex;
			flex-direction: column;
		}

		.divided {
			margin-block-start: var(--space-1);
			padding-block-start: var(--space-1);
			border-block-start: var(--border-thin) solid var(--color-outline-variant);
		}
	}
</style>

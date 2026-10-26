<script lang="ts">
	/*
	 * The notice that recommends a second factor to an Owner who has none
	 * (#1533, ADR-0026's amendment for #1492). The Practice overview shows
	 * it after First Value, her first Staff Invitation, and Stripe
	 * connected; the BFF decides when.
	 *
	 * It states one fact about her account, says what the second step is
	 * and why an Owner has one, names the five acts that need it, and has
	 * one link, to enrollment. It is no checklist and reads no use of hers
	 * (ADR-0046 P5). She can close it (ADR-0043: a notice she cannot close
	 * is the product chasing her); the close is the caller's, because the
	 * BFF keeps it.
	 *
	 * A region with a heading, GOV.UK's notification banner for important
	 * information: it is on the page when she arrives, so it is no live
	 * region and takes no focus. `SecondFactorNeeded` (#1532) is the same
	 * product name and the same link, on the five acts' own screens.
	 */
	import { resolve } from '$app/paths';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	interface Properties {
		/**
		 * The screen she is on. Enrollment sends her back to it.
		 */
		returnTo: string;
		/** Records the close. A rejection keeps the notice on screen and
		 * says the close did not work. */
		onClose: () => Promise<void>;
	}

	let { returnTo, onClose }: Properties = $props();

	const headingId = 'second-factor-recommendation-heading';
	const href = $derived(`${resolve('/(signed-out)/mfa/enroll')}?returnTo=${encodeURIComponent(returnTo)}`);

	let isClosing = $state(false);
	let error = $state('');

	async function handleClose() {
		isClosing = true;
		error = '';
		try {
			await onClose();
		} catch {
			error = 'Could not close this message. Try again.';
		} finally {
			isClosing = false;
		}
	}
</script>

<section aria-labelledby={headingId}><!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
	<stack-l space="var(--space-3)">
		<Heading level={2} variant="card" id={headingId} text="Add a second step when you sign in" />
		<Text
			text="You sign in with your password only. Two-factor authentication adds a second step: after your password, you type a code from an app on your phone. Then a stolen password is not enough to get into your account."
		/>
		<Text
			text="As a Practice Owner, you can see every Client's records and decide who gets in. You need two-factor authentication before you can:"
		/>
		<ul class="prose-list">
			<li>download all of your Practice's data</li>
			<li>delete your Practice</li>
			<li>erase a Client's records</li>
			<li>help a Staff member who is locked out get back in</li>
			<li>make two-factor authentication a must for all your Staff</li>
		</ul>
		{#if error}
			<Notice variant="error" message={error} />
		{/if}
		<cluster-l space="var(--space-3)" align="center">
			<Link {href} label="Set up two-factor authentication" />
			<Button variant="secondary" label="Close" loading={isClosing} onClick={handleClose} />
		</cluster-l>
	</stack-l>
</section>

<style>
	@layer components {
		/* The info Notice's border and tint, as a region that holds a
		   heading, a list and two controls rather than one sentence. */
		section {
			padding: var(--space-4);
			border: var(--border-thin) solid var(--color-info);
			border-radius: var(--radius);
			background-color: color-mix(in oklab, var(--color-info) 12%, var(--color-surface));
		}

		ul {
			margin: 0;
			padding-inline-start: var(--space-6);
			font-family: var(--font-family-base);
			font-size: var(--text-body-size);
			line-height: var(--text-body-leading);
			color: var(--color-on-surface);
		}
	}
</style>

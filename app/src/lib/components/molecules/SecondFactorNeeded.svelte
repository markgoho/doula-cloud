<script lang="ts">
	/*
	 * What one of the five second-factor acts shows in place of its
	 * control when the signed-in session has no second factor (#1532,
	 * ADR-0026's amendment for #1492): the archive, starting Practice
	 * deletion, erasing a Client, vouching for a Staff member, and turning
	 * on "require MFA for all staff". The BFF refuses each of them without
	 * one; this tells her so before she tries, and gives her the way to
	 * fix it.
	 *
	 * Extracted at its first five consumers, which say the same thing with
	 * a different act named. The words follow the act's own screen: the
	 * product calls the thing "two-factor authentication" on enrollment
	 * and on the account screen, so it does here.
	 *
	 * `returnTo` is the screen she is on. `/mfa/enroll` sends her back to
	 * it once she has enrolled, the same way it already does for the
	 * Practice layout's own refusal, and the Practice layout's load reads
	 * the new session as it lands, so the act's control is there.
	 */
	import { resolve } from '$app/paths';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';

	interface Properties {
		/** One sentence naming the act: "You need two-factor authentication
		 * before you can delete this Practice." */
		message: string;
		returnTo: string;
	}

	let { message, returnTo }: Properties = $props();

	const href = $derived(`${resolve('/(signed-out)/mfa/enroll')}?returnTo=${encodeURIComponent(returnTo)}`);
</script>

<stack-l space="var(--space-2)">
	<Notice variant="info" {message} />
	<Link {href} label="Set up two-factor authentication" />
</stack-l>

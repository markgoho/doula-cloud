<!--
/on-the-list (#1799): Buttondown's "After confirming" redirect, the page a
person lands on after she clicks "Confirm my email". Confirming sends the
Welcome email at once, and that email carries the pilot offer (#361), so
the page says it is on its way. The Promotions line is the one the
founder approved for the Welcome email itself.

The pilot terms are not linked here: /pilot-terms stays unlisted (#444),
and the waitlist form's pilot box (#2179) and the Welcome email carry the
link. Unlisted, as /pilot-terms is:
noindex, and left out of sitemap.xml.

The redirect set in Buttondown is https://doula.cloud/on-the-list#_
(#1897), for the reason /check-your-inbox gives: Buttondown can append
"?email_address=<address>", and behind the "#" it never reaches Firebase
Hosting.
-->
<script lang="ts">
	import SignedNote from '#lib/components/SignedNote.svelte';
	import { PRODUCT_NAME, SITE_ORIGIN } from '#lib/product.js';

	const title = "You're on the list";
</script>

<svelte:head>
	<title>{title} | {PRODUCT_NAME}</title>
	<meta name="robots" content="noindex, nofollow" />
	<link rel="canonical" href="{SITE_ORIGIN}/on-the-list" />
</svelte:head>

<SignedNote {title}>
	<p>
		Thanks for confirming. A welcome email from me is on its way, with a small pilot you can join before
		{PRODUCT_NAME} opens in January 2027.
	</p>
	<p>If it lands in your Promotions tab, drag it to Primary so you see the next one.</p>
</SignedNote>

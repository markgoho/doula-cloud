<!--
/check-your-inbox (#1799): Buttondown's "After subscribing" redirect, the
page a person lands on after the waitlist form. Sign-up is double opt-in
(ADR-0014), so she is not on the list yet, and the page says so. The copy
is #362's confirmation, with "your email" for {address}: a static page
with no script cannot read the address. #362's last line, "Then nothing
from me until January.", is left out, because confirming now starts the
Welcome email (#361).

Unlisted, as /pilot-terms is: noindex, and left out of sitemap.xml.
-->
<script lang="ts">
	import SignedNote from '#lib/components/SignedNote.svelte';
	import { PRODUCT_NAME, SITE_ORIGIN } from '#lib/product.js';

	const title = 'Check your inbox';
</script>

<svelte:head>
	<title>{title} | {PRODUCT_NAME}</title>
	<meta name="robots" content="noindex, nofollow" />
	<link rel="canonical" href="{SITE_ORIGIN}/check-your-inbox" />
</svelte:head>

<SignedNote {title}>
	<p>I sent a confirmation link to your email. Click it and you're on the list. Until you do, you're not.</p>
</SignedNote>

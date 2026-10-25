<!--
/check-your-inbox (#1799): Buttondown's "After subscribing" redirect, the
page a person lands on after the waitlist form. Sign-up is double opt-in
(ADR-0014), so she is not on the list yet, and the page says so. The copy
is #362's confirmation, with "your email" for {address}: a static page
with no script cannot read the address. #362's last line, "Then nothing
from me until January.", is left out, because confirming now starts the
Welcome email (#361).

Unlisted, as /pilot-terms is: noindex, and left out of sitemap.xml.

The redirect set in Buttondown is https://doula.cloud/check-your-inbox#_,
not the bare path (#1897). Buttondown appends "?email_address=<address>"
to the redirect, and no setting stops it. Behind the "#", that text is
part of the fragment, which a browser never sends to a server, so
Firebase Hosting receives only /check-your-inbox. The privacy page says
Firebase Hosting receives your IP address, and that stays true. Keep
the "#_" if this redirect is ever set again.
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

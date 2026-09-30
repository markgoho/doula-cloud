<!--
The Terms of Service (#1556). Copy, and the record behind each clause:
docs/copy/terms-of-service-page.md. A change to a sentence here is a
change there, and a new version in site/src/lib/legal.ts and
api/internal/legal in the same commit.

Indexed, unlike /pilot-terms: a Practice has to be able to find the terms
she agrees to, and a crawler is welcome to list them. So no robots meta,
a canonical address, and an entry in sitemap.xml.

The Refunds section is word for word the one on /pilot-terms, and the
spec beside this file fails when the two drift (ADR-0054).
-->
<script lang="ts">
	import PageSection from '#lib/components/PageSection.svelte';
	import ReadingPage from '#lib/components/ReadingPage.svelte';
	import VersionHistory from '#lib/components/VersionHistory.svelte';
	import { TERMS, currentVersion } from '#lib/legal.js';
	import { formatUpdated } from '#lib/practicePage.js';
	import { PRODUCT_NAME, SITE_ORIGIN } from '#lib/product.js';

	const effective = currentVersion(TERMS).effective;
	const effectiveDate = formatUpdated(effective);
</script>

<svelte:head>
	<title>{TERMS.name} | {PRODUCT_NAME}</title>
	<meta
		name="description"
		content="The agreement between a practice and Elephantine LLC, which operates {PRODUCT_NAME}: what a Credit costs, refunds, leaving, and how the terms change."
	/>
	<link rel="canonical" href="{SITE_ORIGIN}{TERMS.path}" />
</svelte:head>

<ReadingPage title={TERMS.name} updated={effective}>
	<stack-l>
		<p>
			These terms are the agreement between your practice and Elephantine LLC, a New York limited
			liability company that operates {PRODUCT_NAME}. In them, “we” and “us” mean Elephantine LLC, and
			“you” means your practice.
		</p>
		<p>
			Your practice agrees to these terms when its Owner creates it, by pressing Create Practice under
			the sentence that links them. The Owner agrees for the practice, and must have the authority to
			do so.
		</p>
		<p>
			This version takes effect on <time datetime={effectiveDate.datetime}>{effectiveDate.text}</time>.
			The history of every version is at the end of this page.
		</p>
	</stack-l>

	<PageSection heading="What {PRODUCT_NAME} is">
		<p>
			{PRODUCT_NAME} is software a doula practice uses to run its business: its clients, engagements,
			visits, plans, contracts, invoices and messages. It is offered to practices in the United States.
		</p>
	</PageSection>

	<PageSection heading="What a Credit costs">
		<p>
			One Credit starts one engagement — a single client relationship centered on one baby, from
			intake through the end of care. Birth and postpartum engagements cost the same.
		</p>
		<p>
			A Credit costs <strong>$20.00</strong>, plus sales tax where it applies. That is the one price,
			and every feature is included in it.
		</p>
		<p>
			There is no subscription, no minimum and no recurring charge. You buy Credits when you want them
			and are charged nothing in between. Credits do not expire, and the oldest are spent first.
		</p>
		<p>
			If the price of a Credit ever rises, we email each Owner at least 60 days before, with the
			reason. A rise never changes a Credit you already bought: it keeps the price you paid for it, and
			until the new price starts you can buy as many Credits as you like at the old one.
		</p>
		<p>When you buy Credits, Stripe takes the payment for us. We never see or store your card number.</p>
	</PageSection>

	<!-- Word for word the Refunds section of /pilot-terms. The spec beside
	     this file holds the two together; a change to one is a change to
	     both and to the /support copy (ADR-0054). -->
	<PageSection heading="Refunds">
		<p>Credits given free of charge are not refundable.</p>
		<p>
			Purchased Credits that have not been spent can be refunded within three years of the date they
			were bought, at the price paid for them and together with any sales tax charged on them, to the
			original payment method. A Credit already used to start an engagement has been spent, and is not
			refundable. To ask for a refund, email us. We do not need a reason.
		</p>
	</PageSection>

	<PageSection heading="Your practice is the merchant">
		<p>
			{PRODUCT_NAME} is not a payment service. When your practice invoices a client, your practice is
			the merchant: it holds its own agreement with Stripe, the money is paid into its own account, and
			it is responsible for the care it provides and for what it charges. We never receive or hold your
			practice's money, and we take no part of any payment a client makes to you.
		</p>
	</PageSection>

	<PageSection heading="Stripe">
		<p>
			Payment processing services for your practice are provided by Stripe and are subject to the <a
				href="https://stripe.com/connect-account/legal/full">Stripe Connected Account Agreement</a
			>, which includes the Stripe Terms of Service. That agreement is between your practice and Stripe.
			Stripe asks your Owner to accept it on Stripe's own pages, when your practice is connected to
			Stripe, and Stripe can change it from time to time.
		</p>
		<p>
			You agree to give accurate and complete information about your practice, and you authorize us to
			share it, and information about the payments your practice takes through {PRODUCT_NAME}, with
			Stripe. Stripe tells us whether your practice's account can take payments.
		</p>
		<p>
			Stripe processes your data as <a href="https://stripe.com/privacy">Stripe's Privacy Policy</a>
			explains.
		</p>
	</PageSection>

	<PageSection heading="Leaving">
		<p>You can leave at any time, and you take everything with you.</p>
		<p>
			<strong>Export.</strong> An Owner can download everything your practice has put into
			{PRODUCT_NAME} as one archive, in formats you can open without {PRODUCT_NAME}. Take it before you
			start deletion: the archive cannot be downloaded while deletion is under way.
		</p>
		<p>
			<strong>Deletion.</strong> An Owner can delete your practice from its settings, with no call and
			no reason asked. Deletion starts a 30-day window. During it, the practice is locked for everyone,
			and an Owner can restore it. We email each Owner 7 days before the window ends. When it ends,
			every client record is erased and the practice is closed.
		</p>
		<p>
			<strong>Credits when you leave.</strong> Ask us for a refund of your unspent purchased Credits
			before you start deletion, as Refunds above says. A Credit still unspent when deletion finishes
			ends with the practice.
		</p>
		<p>
			<strong>What stays after deletion.</strong> We keep our own records of your practice: its name and
			details, the Credits it bought and spent, the record of what was done and by whom, with each
			client's details made unreadable, and the record of when your practice agreed to these terms and
			to the Privacy Policy. We keep them for our accounts and taxes, and to answer a question or a
			dispute about your practice after it has gone. Your practice's Stripe account is its own, and
			deletion does not close it.
		</p>
		<p>Nothing in these terms is there to make leaving cost you more.</p>
	</PageSection>

	<PageSection heading="What your practice is responsible for">
		<p>
			Your practice is responsible for the care it gives, for what it charges its clients, and for what
			it tells its clients about the records it keeps on them. {PRODUCT_NAME} gives no medical advice,
			and it is not responsible for the care your practice gives.
		</p>
		<p>
			Your practice decides what it records about its clients and who on its staff can see it. It must
			have the right to put that information into {PRODUCT_NAME}, and it must answer a client who asks
			to see, correct or erase their record.
		</p>
		<p>
			Each person at your practice must keep their sign-in details private. Tell us at once if you
			think someone else has used them.
		</p>
	</PageSection>

	<PageSection heading="What {PRODUCT_NAME} must not be used for">
		<p>Do not use {PRODUCT_NAME} to:</p>
		<ul>
			<li>break the law, or help someone else break it;</li>
			<li>store information about a person that your practice has no right to hold;</li>
			<li>send messages a person has not agreed to receive, or harass anyone;</li>
			<li>
				try to reach data that is not your practice's, or test, break or overload the security of
				{PRODUCT_NAME};
			</li>
			<li>resell {PRODUCT_NAME}, or copy it.</li>
		</ul>
		<p>
			If a practice does any of these, we can suspend or close its access. Unless the law or someone's
			safety stops us, we tell its Owners first and give them time to download their export.
		</p>
	</PageSection>

	<PageSection heading="HIPAA and insurance billing">
		<p>
			{PRODUCT_NAME} is not a business associate under HIPAA, and it signs no business associate
			agreement today.
		</p>
		<p>
			A practice that sends a claim electronically to Medicaid or to an insurer can become a covered
			entity under HIPAA. Tell us before your practice sends its first electronic claim, so that we can
			tell you what {PRODUCT_NAME} can and cannot do for a covered entity. Until then, do not use
			{PRODUCT_NAME} as if it were your business associate.
		</p>
	</PageSection>

	<PageSection heading="When these terms change">
		<p>
			We decide for each new version whether the change is material. A material change is a change to
			what your practice pays, what it gets, or what happens to its data. A corrected typing error is
			not.
		</p>
		<p>
			For a material change, we email each Owner at least 30 days before the new version takes effect,
			and say what changes. After that date, the first Owner who signs in is asked to agree to the new
			version. Your practice works as usual until then.
		</p>
		<p>A change never applies to a dispute, a claim or a purchase that came before it.</p>
		<p>
			If you do not want to agree to a change, you can leave before it takes effect, with your export
			and a refund of your unspent purchased Credits.
		</p>
		<p>
			A change that is not material, such as a clearer sentence or a new postal address, takes effect on
			the date of its version. The history below records every version.
		</p>
	</PageSection>

	<PageSection heading="Warranty">
		<p>
			We work to keep {PRODUCT_NAME} running and your data safe, and we fix what goes wrong. We cannot
			promise that it will never be unavailable or never have an error. {PRODUCT_NAME} is provided as it
			is, and we give no warranty beyond what these terms say, except where the law does not allow us to
			leave one out.
		</p>
		<p>Keep your own copy of what matters to your practice. The export is there for that.</p>
	</PageSection>

	<PageSection heading="Liability">
		<p>
			Neither your practice nor we are responsible to the other for a loss that could not reasonably
			have been foreseen, or for lost profits or lost business.
		</p>
		<p>
			Our total liability to your practice for all claims about {PRODUCT_NAME} is limited to the amount
			your practice paid us in the 12 months before the claim arose, or $100, whichever is more.
		</p>
		<p>These limits do not apply to a refund we owe you, or where the law does not allow them.</p>
	</PageSection>

	<PageSection heading="Disputes">
		<p>These terms are governed by the law of the State of New York.</p>
		<p>
			A lawsuit about these terms or about {PRODUCT_NAME} goes to the state or federal courts for Monroe
			County, New York, and your practice and we both agree to those courts.
		</p>
		<p>
			Nothing in these terms takes away your right to go to court, on your own or together with others.
			Before either of us goes to court, we ask that you write to us first, so that we can try to put it
			right.
		</p>
	</PageSection>

	<PageSection heading="Everything else">
		<p>
			These terms and the <a href="/privacy">Privacy Policy</a> are the whole agreement between your
			practice and us about {PRODUCT_NAME}. If a court finds that part of them cannot be enforced, the
			rest stays in force. If we do not enforce a term at once, we have not given it up.
		</p>
	</PageSection>

	<PageSection heading="Contact us">
		<p>
			Email <a href="mailto:hello@doula.cloud">hello@doula.cloud</a>. That address reaches a person.
		</p>
	</PageSection>

	<VersionHistory legalDocument={TERMS} />
</ReadingPage>

<style>
	ul {
		padding-inline-start: var(--space-6);
	}
</style>

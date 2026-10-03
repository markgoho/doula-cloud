<!--
The Privacy Policy (#1556). Copy, and the code or record each claim was
checked against: docs/copy/privacy-policy-page.md. A sentence about a
company, a cookie or how long something is kept is written only once it
has been checked there; a change to one is a new version in
site/src/lib/legal.ts and api/internal/legal in the same commit.

Indexed and conspicuously posted: CalOPPA asks for the second, and the
site footer on every page links here.
-->
<script lang="ts">
	import PageSection from '#lib/components/PageSection.svelte';
	import ReadingPage from '#lib/components/ReadingPage.svelte';
	import VersionHistory from '#lib/components/VersionHistory.svelte';
	import { PRIVACY, currentVersion } from '#lib/legal.js';
	import { formatUpdated } from '#lib/practicePage.js';
	import { PRODUCT_NAME, SITE_ORIGIN } from '#lib/product.js';

	const effective = currentVersion(PRIVACY).effective;
	const effectiveDate = formatUpdated(effective);
</script>

<svelte:head>
	<title>{PRIVACY.name} | {PRODUCT_NAME}</title>
	<meta
		name="description"
		content="What {PRODUCT_NAME} holds about a practice's owners, its staff, its clients and a visitor to this site, who else receives it, how long it is kept, and how to have it erased."
	/>
	<link rel="canonical" href="{SITE_ORIGIN}{PRIVACY.path}" />
</svelte:head>

<ReadingPage title={PRIVACY.name} updated={effective}>
	<stack-l>
		<p>
			This policy says what Elephantine LLC, which operates {PRODUCT_NAME}, holds about each person who
			uses {PRODUCT_NAME} or visits this site: where it comes from, what it is used for, who else
			receives it, how long it is kept, and how you see it, correct it or have it erased. “We” and “us”
			mean Elephantine LLC.
		</p>
		<p>
			This version takes effect on <time datetime={effectiveDate.datetime}>{effectiveDate.text}</time>.
			The history of every version is at the end of this page.
		</p>
	</stack-l>

	<PageSection heading="Who decides about your data">
		<p>
			A practice's records about its clients belong to the practice. The practice decides what it
			records about a client, who on its staff sees it, and when it is erased. We hold those records for
			the practice, and use them only to run {PRODUCT_NAME} for it.
		</p>
		<p>
			For the account of each person who owns or works at a practice, and for a visitor to this site,
			we decide, and this policy says how.
		</p>
	</PageSection>

	<PageSection heading="What we never do">
		<ul>
			<li>We do not sell anyone's data.</li>
			<li>We do not use it for advertising, and {PRODUCT_NAME} shows no advertisements.</li>
			<li>
				No email we send is tracked for opens or clicks. A message from your doula is only a message.
			</li>
			<li>
				We do not study what a named person does in {PRODUCT_NAME}. The record of who changed what is
				there for the practice to read, and the only use we measure is how many Credits each practice
				bought and spent.
			</li>
		</ul>
	</PageSection>

	<PageSection heading="If you own a practice">
		<p>When you create a practice, and while you run it, we hold:</p>
		<ul>
			<li>
				your name and email address, and the password and any second sign-in factor you set. Google
				Identity Platform checks your password for us, and we do not store it;
			</li>
			<li>your practice's name, its time zone, and what else you enter about it;</li>
			<li>
				the Credits your practice buys and spends. Stripe takes the payment, and we never see your card
				number;
			</li>
			<li>your practice's Stripe account number, and whether Stripe says it can take payments;</li>
			<li>
				your practice's public page, if you publish one. It is public by design, and shows what you put
				on it, including the name and email address you give as its contact;
			</li>
			<li>
				the date, the time and the IP address when your practice agrees to our <a href="/terms"
					>Terms of Service</a
				> and to this policy;
			</li>
			<li>what you tell us through Feedback, with the name and version of your browser;</li>
			<li>a record of each change you make, with who made it and when.</li>
		</ul>
		<p>
			We use it to run {PRODUCT_NAME} for your practice, to charge for Credits, to email you about your
			practice's account, and to answer you.
		</p>
	</PageSection>

	<PageSection heading="If you work at a practice">
		<p>
			When a practice invites you to work with it, we hold your name and email address, your password
			and second sign-in factor as the section above says, the practices you work at and your role at
			each, and what you do at each: the visits you are given, the messages you write, and the changes
			you make, with when you made them. We also hold what you tell us through Feedback, with the name
			and version of your browser.
		</p>
		<p>
			You are one person with one login, even if you work at more than one practice. Each practice sees
			your name, your email address and the work you do for it. It does not see the other practices you
			work at, or what you do there.
		</p>
		<p>
			You can delete your login yourself. It is deleted at once, and your place at every practice ends.
			Your name and email address are replaced, and the work you did stays in each practice's records
			under “Deleted Staff Member”. If you are the last Owner of a practice, another person must become
			an Owner, or the practice must be deleted, first.
		</p>
		<p>
			The Feedback you sent us stays when your login is deleted, and when a practice you work at is
			deleted. Once your login is deleted, it no longer names you. It is deleted 24 months after you
			sent it.
		</p>
	</PageSection>

	<PageSection heading="If you are a client of a practice">
		<p>
			Your doula's practice uses {PRODUCT_NAME} to keep its records about you. This section is written to
			you.
		</p>
		<p>
			Your practice decides what it records. That can include your name, contact details, address and
			date of birth; what it records about your pregnancy, birth and care; your visits; your plans, such
			as a birth plan; the contracts you sign; your invoices and payments; and the messages and files
			you and your doula send each other. When you sign a contract, we record the time and the IP address
			it was signed from.
		</p>
		<p>You sign in to your practice's portal with a link we email to you. There is no password.</p>
		<p>
			When you pay an invoice, the payment form comes from Stripe, and your card details go straight to
			Stripe. We never see or store a card number. Stripe receives your name and email address with each
			invoice, and collects information about your device to prevent fraud.
		</p>
		<p>
			<strong>To see, correct or erase your record, ask your practice.</strong> It is your practice's
			record, and your practice decides. If your practice does not answer you, write to us at
			<a href="mailto:hello@doula.cloud">hello@doula.cloud</a>. We cannot change a practice's record
			ourselves, but we will send your request to your practice and ask it to answer you.
		</p>
		<p>
			When your practice erases your record, your name, contact details, address, date of birth and the
			details it recorded about you in its own fields are removed, Stripe's customer record of you is
			deleted, and the history of changes to those details becomes unreadable. What was written in
			messages, contracts and notes stays, because those are your practice's own records of the care it
			gave and what it charged.
		</p>
		<p>
			You can tell us what you think of {PRODUCT_NAME} through Feedback in the portal. It comes to us,
			never to your practice. We keep what you send, with the name and version of your browser, and
			erase it when your practice erases your record or is deleted. If more than one practice keeps a
			record of you, Feedback you sent from a portal page that is not about one practice is erased when
			the last of them erases your record.
		</p>
	</PageSection>

	<PageSection heading="If you visit this site">
		<p>
			This site sets no cookies and runs no scripts. It does not count visits today. If it starts to, it
			will count them with a service that sets no cookie and cannot identify you, and this policy will
			name that service first.
		</p>
		<p>Firebase Hosting, a Google service, delivers each page, and receives your IP address to do so.</p>
		<p>
			This site has no waitlist form today. If you join the waitlist once it has one, your email address
			and first name are kept by Buttondown, the service that runs the list. After you confirm your
			address, the list writes to you once, when {PRODUCT_NAME} opens in January 2027.
		</p>
		<p>
			{PRODUCT_NAME} does not follow you across other sites, so it treats every visit the same whether or
			not your browser sends a Do Not Track signal.
		</p>
	</PageSection>

	<PageSection heading="Who else receives data">
		<p>These companies receive data to run {PRODUCT_NAME}, and each receives only what its job needs:</p>
		<ul>
			<li>
				<strong>Google Cloud</strong> runs {PRODUCT_NAME} and stores all of its data, in the United
				States: the database, files such as contract documents and message attachments, and the jobs
				that send its email.
			</li>
			<li>
				<strong>Google Identity Platform</strong> checks the sign-in of each person who owns or works at
				a practice: the email address, the password and the second sign-in factor.
			</li>
			<li>
				<strong>Firebase Hosting</strong>, a Google service, delivers the {PRODUCT_NAME} app and this
				site, and receives the IP address of each request.
			</li>
			<li>
				<strong>Mailgun</strong> sends {PRODUCT_NAME}'s email. It receives each address and message, and
				deletes its logs of them within 30 days.
			</li>
			<li>
				<strong>Stripe</strong> takes payments: a practice's payment for Credits, and a client's payment
				of an invoice. It receives what each payment needs, including a client's name and email address
				with an invoice, and it holds each practice's own Stripe account. Stripe processes data as <a
					href="https://stripe.com/privacy">Stripe's Privacy Policy</a
				> explains.
			</li>
			<li>
				<strong>Your browser's push service</strong> — Google, Apple or Mozilla, depending on your
				browser — carries the signal that tells your device a new message is waiting. The signal says
				only that something is waiting, never what it is.
			</li>
			<li>
				<strong>GitHub</strong> keeps a note of each piece of Feedback in a private repository, so that we
				can sort it. The note holds only:
				<ul>
					<li>the kind of Feedback;</li>
					<li>the kind of page it was sent from, never that page's address;</li>
					<li>the version of the app, the width of the screen, and the name and version of the browser;</li>
					<li>the time it was sent;</li>
					<li>the role of the person who sent it, such as Owner or Client;</li>
					<li>a link that only we can open.</li>
				</ul>
				It never holds the words a person typed, a person's name, or a practice's name. The note names
				nobody, so it stays after the Feedback is erased or deleted.
			</li>
		</ul>
		<p>
			A company is added to this list, in a new version of this policy, before it receives anyone's
			data.
		</p>
	</PageSection>

	<PageSection heading="Cookies and storage on your device">
		<p>
			{PRODUCT_NAME} sets one cookie, <code>__session</code>, which keeps you signed in. It is used for
			nothing else, and it ends when you sign out or your session expires.
		</p>
		<p>
			When a person who owns or works at a practice signs in, Google's sign-in library keeps a record of
			the sign-in in the browser's own storage until they sign out. The app also keeps a form you have
			not sent yet, such as a new client's intake, in the browser tab's own storage until the tab is
			closed.
		</p>
		<p>This site sets no cookies.</p>
	</PageSection>

	<PageSection heading="How long we keep data">
		<ul>
			<li>We keep a practice's records for as long as the practice uses {PRODUCT_NAME}.</li>
			<li>A client's record is erased at once when the practice erases it, as described above.</li>
			<li>
				A practice's records are erased at the end of its 30-day deletion window, as the <a
					href="/terms">Terms of Service</a
				> describe.
			</li>
			<li>A login is deleted at once when its owner deletes it, as described above.</li>
			<li>
				The database is backed up each day, and each backup is kept for 7 days. Something erased can
				stay in a backup for up to 7 days after it was erased, and is then gone.
			</li>
			<li>Mailgun deletes its logs of each email within 30 days.</li>
			<li>
				Feedback is deleted 24 months after it was sent, unless it is erased sooner as described above.
				GitHub's note of it names nobody, and stays.
			</li>
		</ul>
		<p>
			Some records stay after a practice or a login is deleted: the practice's own name and details, the
			Credits it bought and spent, the record of what was done and by whom, with each client's details
			made unreadable, and the record of when an Owner agreed to the Terms of Service and to this policy.
			We keep them for our accounts and taxes, and because the record of an agreement is needed most
			after the person who agreed has gone.
		</p>
	</PageSection>

	<PageSection heading="How we protect data">
		<p>We keep a program of safeguards, as New York's SHIELD Act asks. Among them:</p>
		<ul>
			<li>every connection to {PRODUCT_NAME} and to this site is encrypted;</li>
			<li>Google Cloud encrypts the data it stores;</li>
			<li>the database itself keeps each practice's records apart from every other practice's;</li>
			<li>each person who owns or works at a practice can add a second sign-in factor;</li>
			<li>
				the history of changes to a client's details is locked with a key of its own, which is destroyed
				when the record is erased.
			</li>
		</ul>
	</PageSection>

	<PageSection heading="If there is a breach">
		<p>
			If someone gets access to data they should not have, we tell each practice whose data is affected,
			without unreasonable delay, so that it can tell its clients. We also tell the people and the
			authorities that the law says we must.
		</p>
	</PageSection>

	<PageSection heading="Children">
		<p>
			{PRODUCT_NAME} is for adults, and a child cannot have an account. A practice's record can name a
			baby. That information comes from the baby's parent or from the practice, and the parent can ask
			the practice about it as the section for clients describes.
		</p>
	</PageSection>

	<PageSection heading="Where this policy applies">
		<p>
			{PRODUCT_NAME} is offered in the United States, and its data is stored there. This policy is
			written for the laws of the United States.
		</p>
	</PageSection>

	<PageSection heading="When this policy changes">
		<p>
			We decide for each new version whether the change is material, as the <a href="/terms"
				>Terms of Service</a
			> describe. A change to what happens to anyone's data is material. For a material change, we email
			each Owner at least 30 days before the new version takes effect.
		</p>
		<p>
			A material change to how we use data we already hold applies to that data only after an Owner of
			the practice has agreed to it. Until then, that data keeps the promises this policy made when it
			was collected.
		</p>
		<p>Every version is posted on this page, and the history below says what changed.</p>
	</PageSection>

	<PageSection heading="Contact us">
		<p>
			Email <a href="mailto:hello@doula.cloud">hello@doula.cloud</a>. That address reaches a person.
		</p>
	</PageSection>

	<VersionHistory legalDocument={PRIVACY} />
</ReadingPage>

<style>
	ul {
		padding-inline-start: var(--space-6);
	}
</style>

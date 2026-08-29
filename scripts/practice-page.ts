/**
 * Rendering a Practice's published page into the Hugo content tree (#441).
 *
 * Pure on purpose: everything here is a value in and a string out, so
 * the page a Practice will see is testable without a database, a
 * network, or a Hugo build. The half that talks to Postgres is
 * sync-practice-pages.ts, and it does nothing but hand rows to these
 * functions.
 *
 * No Bun APIs are used in this file. It is imported by the test as well
 * as by the sync script, and a test runner is a poor place to discover
 * that a global was only defined in one of them.
 */

/** One published page, as the sync script reads it out of Postgres. */
export interface PracticePage {
  /** The path segment of doula.cloud/p/<slug>, assigned once (00046). */
  slug: string;
  /** The Practice's name, and the business name Stripe asks for. */
  name: string;
  /** What she offers, in her own words. Up to 500 characters. */
  serviceDescription: string;
  /** Her cancellation or refund position. Up to 500 characters. */
  cancellationPolicy: string;
  /** The name of the Owner who published, shown as the contact. */
  supportName: string;
  /** That Owner's email address -- how a Client reaches the Practice. */
  supportEmail: string;
  /** When the page was last published, RFC 3339. */
  publishedAt: string;
}

/**
 * The privacy statement every hosted page carries.
 *
 * Stripe never asks for one -- #421 walked the hosted flow and found no
 * privacy policy field anywhere in it, and no enforcement afterwards.
 * This is here for the Client, who is about to pay four figures to a
 * business she found through a page on someone else's domain, and who
 * may reasonably wonder who is holding what.
 *
 * One shared block rather than a field on the form: it describes what
 * Doula Cloud does with the data, which is the same for every Practice,
 * and a Practice asked to write her own would be asked to write ours.
 */
export const PRIVACY_STATEMENT = [
  "This page is published by Doula Cloud on behalf of the practice named above.",
  "Doula Cloud provides the software the practice uses to run its business, and stores the practice's records on its behalf.",
  "If you are a client of this practice, the practice decides what it records about you and how long it keeps it; write to the practice at the address above to ask what it holds or to ask it to correct something.",
  "Doula Cloud does not sell any of it, and does not use it to advertise to you.",
  "Payments are processed by Stripe, which handles your card details directly; Doula Cloud never sees or stores a card number.",
].join(" ");

/**
 * The directory a page is written to, relative to the Hugo content root.
 *
 * A leaf bundle -- `p/<slug>/index.md` -- rather than `p/<slug>.md`, so
 * a page that later needs an image has somewhere to put it without the
 * URL moving. The URL Stripe holds is the one thing here that must never
 * move (#382).
 */
export function pageDirectory(slug: string): string {
  return `p/${slug}`;
}

/**
 * Renders one page as a Hugo leaf bundle's index.md.
 *
 * Every value a Practice typed travels in JSON front matter and none of
 * it in the markdown body. That is a safety decision, not a stylistic
 * one: the body would be run through a markdown renderer, where her
 * cancellation policy's stray asterisks, brackets and angle brackets
 * become formatting or vanish, and where the only thing standing between
 * a pasted `<script>` and the page is Hugo's `unsafe` setting being left
 * at its default. Front matter is data. The layout prints it, and Go's
 * html/template escapes it for the context it lands in.
 *
 * JSON rather than YAML front matter for the same reason: JSON.stringify
 * is a complete escaper for arbitrary text, and hand-rolled YAML quoting
 * is a class of bug rather than a function.
 */
export function renderPage(page: PracticePage): string {
  const frontMatter = {
    title: page.name,
    // The layout selects on this; nothing else in the site is type "p".
    type: "p",
    url: `/p/${page.slug}/`,
    // Hugo's `date` drives the sitemap's lastmod, which is the honest
    // thing for a page that changes when she republishes it.
    date: page.publishedAt,
    serviceDescription: page.serviceDescription,
    cancellationPolicy: page.cancellationPolicy,
    supportName: page.supportName,
    supportEmail: page.supportEmail,
    privacyStatement: PRIVACY_STATEMENT,
  };
  // Two spaces and a trailing newline: the file is generated, but it is
  // also the thing anyone debugging a rejected Stripe review will read.
  return `${JSON.stringify(frontMatter, null, 2)}\n`;
}

/**
 * The section page for `p/`.
 *
 * There is deliberately no public directory of Practices: nobody decided
 * to publish one, and a page listing every Practice on the platform is a
 * product decision rather than a side effect of how the pages are
 * stored. Hugo would otherwise publish an empty /p/ and put it in the
 * sitemap, pointing a crawler -- and a Stripe reviewer following it --
 * at nothing.
 *
 * `render: never` suppresses the section's own output without touching
 * its children, which keep their own build settings and their own
 * sitemap entries.
 */
export function renderSectionIndex(): string {
  return `${JSON.stringify({ title: "Practices", build: { render: "never", list: "never" } }, null, 2)}\n`;
}

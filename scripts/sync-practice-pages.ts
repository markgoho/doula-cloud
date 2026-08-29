/**
 * Pre-build step: write every published Practice page into the Hugo
 * content tree (#441).
 *
 * Runs before `hugo` in the same `bun run build` the deploy workflow
 * already calls. Reads the Practices that chose a hosted page (#440's
 * `practice_websites`, mode 'hosted') and writes
 * `hugo/content/p/<slug>/index.md` for each. The result is a static file
 * behind Firebase Hosting's CDN: no dynamic path on doula.cloud, no
 * Cloud Run rewrite, nothing to be up at the moment a Client or a Stripe
 * reviewer opens the page.
 *
 * Usage:
 *   SYNC_PRACTICE_PAGES=required DATABASE_URL=... bun scripts/sync-practice-pages.ts
 *
 * ## Why the switch, and why it defaults to off
 *
 * This script prunes. It has to -- a Practice who switches back to her
 * own website must lose her page on the next build, and the only way a
 * generated tree forgets something is by being rebuilt from scratch. So
 * "no rows" and "could not reach the database" would produce the same
 * output if this connected opportunistically: an empty tree, deleting
 * every live page whose URL Stripe is holding, against a review #382
 * established is ongoing.
 *
 * There is therefore no "connect if you can". Either SYNC_PRACTICE_PAGES
 * is `required`, in which case a database that cannot be reached fails
 * the build -- and the deploy workflow's build/deploy split means a
 * failed build produces no artifact and the live site stays exactly as
 * it was -- or the variable is absent, in which case this touches
 * nothing at all. The second case is a local `bun run build` and the PR
 * preview workflow, neither of which has a credential, and neither of
 * which should silently publish a stale or empty set of pages.
 */

import { mkdir, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { SQL } from "bun";
import {
  type PracticePage,
  pageDirectory,
  renderPage,
  renderSectionIndex,
} from "./practice-page";

const CONTENT_ROOT = path.resolve(
  import.meta.dirname,
  "../hugo/content",
);

/** The whole generated tree. Rebuilt from the database on every run. */
const PAGES_ROOT = path.join(CONTENT_ROOT, "p");

/**
 * The published pages, and the Owner each one names as its contact.
 *
 * The contact is chosen by one rule rather than two: order the
 * Practice's current Owners by whether each is the person who published
 * the page, then by how long she has been an Owner, and take the first.
 * That is the publisher when she is still an Owner -- she is the person
 * who decided what the page says, and #440 recorded her on the event row
 * for exactly that reason -- and the longest-standing Owner when she has
 * since left, so a page does not print a dead address the moment someone
 * moves on.
 *
 * `practice_website_events` is filtered to mode 'hosted': the newest
 * event may be a switch away to her own website, which is not the act
 * that published what is on the page.
 */
const PUBLISHED_PAGES = `
  SELECT w.slug,
         p.name,
         w.service_description,
         w.cancellation_policy,
         support.name  AS support_name,
         support.email AS support_email,
         published.created_at AS published_at
    FROM practice_websites w
    JOIN practices p ON p.id = w.practice_id
    LEFT JOIN LATERAL (
           SELECT e.actor_staff_id, e.created_at
             FROM practice_website_events e
            WHERE e.practice_id = w.practice_id
              AND e.mode = 'hosted'
            ORDER BY e.created_at DESC
            LIMIT 1
         ) published ON true
    LEFT JOIN LATERAL (
           SELECT s.name, s.email
             FROM practice_memberships m
             JOIN staff s ON s.id = m.staff_id
            WHERE m.practice_id = w.practice_id
              AND 'owner' = ANY (m.roles)
            ORDER BY (s.id = published.actor_staff_id) DESC, m.created_at
            LIMIT 1
         ) support ON true
   WHERE w.mode = 'hosted'
   ORDER BY w.slug
`;

interface PublishedRow {
  slug: string;
  name: string;
  service_description: string | null;
  cancellation_policy: string | null;
  support_name: string | null;
  support_email: string | null;
  published_at: Date | null;
}

/**
 * Turns a database row into the page it describes, refusing a row that
 * cannot make a complete one.
 *
 * A hosted row without its two facts is impossible (00045's CHECK), and
 * a Practice without an Owner is impossible too -- so a row that fails
 * here means something is wrong that publishing a half-page would only
 * hide. Stripe's written standard asks for a support contact by name;
 * a page missing one is a page that fails the review it exists to pass.
 */
export function toPage(row: PublishedRow): PracticePage {
  const missing = (["service_description", "cancellation_policy", "support_name", "support_email"] as const)
    .filter((field) => !row[field]);
  if (missing.length > 0) {
    throw new Error(
      `practice page ${row.slug} is missing ${missing.join(", ")}; refusing to publish an incomplete page`,
    );
  }
  return {
    slug: row.slug,
    name: row.name,
    serviceDescription: row.service_description as string,
    cancellationPolicy: row.cancellation_policy as string,
    supportName: row.support_name as string,
    supportEmail: row.support_email as string,
    publishedAt: (row.published_at ?? new Date()).toISOString(),
  };
}

/** Reads every published page, as the site_builder role and nothing more. */
async function readPublishedPages(databaseUrl: string): Promise<PracticePage[]> {
  const sql = new SQL({ url: databaseUrl, max: 1 });
  try {
    // SET ROLE inside a transaction, which pins one connection, so the
    // read genuinely happens under the role 00046 created. Explicit
    // rather than relying on the login user inheriting it: if the grant
    // is ever missing this fails here, loudly, instead of quietly
    // returning zero rows through a policy that did not match and
    // deleting every page.
    return await sql.begin(async (tx) => {
      await tx`SET ROLE site_builder`;
      const rows = (await tx.unsafe(PUBLISHED_PAGES)) as PublishedRow[];
      return rows.map(toPage);
    });
  } finally {
    await sql.close();
  }
}

/**
 * Replaces the generated tree with exactly the pages given.
 *
 * Removed wholesale and rewritten rather than reconciled, because the
 * requirement is a tree that matches the database and not a tree that
 * has been patched toward it. The directory is generated output and is
 * gitignored, so there is nothing here for the removal to lose.
 */
export async function writePages(pages: PracticePage[], root = PAGES_ROOT): Promise<void> {
  await rm(root, { recursive: true, force: true });
  await mkdir(root, { recursive: true });
  await writeFile(path.join(root, "_index.md"), renderSectionIndex(), "utf8");
  for (const page of pages) {
    const dir = path.join(path.dirname(root), pageDirectory(page.slug));
    await mkdir(dir, { recursive: true });
    await writeFile(path.join(dir, "index.md"), renderPage(page), "utf8");
  }
}

async function main(): Promise<void> {
  if (process.env.SYNC_PRACTICE_PAGES !== "required") {
    console.log(
      "sync-practice-pages: SYNC_PRACTICE_PAGES is not 'required'; leaving hugo/content/p alone.",
    );
    return;
  }

  const databaseUrl = process.env.DATABASE_URL;
  if (!databaseUrl) {
    throw new Error(
      "SYNC_PRACTICE_PAGES=required but DATABASE_URL is unset; refusing to build a site with no Practice pages in it",
    );
  }

  const pages = await readPublishedPages(databaseUrl);
  await writePages(pages);
  console.log(`sync-practice-pages: wrote ${pages.length} published Practice page(s).`);
}

// Only when run as a program. Importing this file from a test must not
// open a database connection.
if (import.meta.main) {
  main().catch((error: unknown) => {
    console.error("sync-practice-pages:", error);
    process.exit(1);
  });
}

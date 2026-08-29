/**
 * Tests for the page a Practice publishes at doula.cloud/p/<slug> (#441).
 *
 * No database. The two halves are split so this is possible: rendering
 * is a value in and a string out, and the only thing that talks to
 * Postgres is one function in sync-practice-pages.ts that hands rows to
 * these. What is proved here is what a Stripe reviewer will read.
 */

import { describe, expect, test } from "bun:test";
import { mkdtemp, readFile, readdir, writeFile, mkdir } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import {
  PRIVACY_STATEMENT,
  type PracticePage,
  pageDirectory,
  renderPage,
  renderSectionIndex,
} from "./practice-page";
import { toPage, writePages } from "./sync-practice-pages";

const page: PracticePage = {
  slug: "rochester-doulas",
  name: "Rochester Doulas",
  serviceDescription: "Birth and postpartum support across Monroe County.",
  cancellationPolicy: "Cancel more than 30 days before the due date for a full refund.",
  supportName: "Maya Chen",
  supportEmail: "maya@rochesterdoulas.com",
  publishedAt: "2026-08-29T14:00:00.000Z",
};

/** The front matter, parsed back out of a rendered page. */
function frontMatter(rendered: string): Record<string, unknown> {
  return JSON.parse(rendered) as Record<string, unknown>;
}

describe("renderPage", () => {
  test("carries every field Stripe's website standard asks for", () => {
    const fm = frontMatter(renderPage(page));

    // #382 read these off Stripe's own written requirements: the
    // business name, a description of the services, a support contact,
    // and a refund or cancellation position.
    expect(fm.title).toBe("Rochester Doulas");
    expect(fm.serviceDescription).toBe(page.serviceDescription);
    expect(fm.cancellationPolicy).toBe(page.cancellationPolicy);
    expect(fm.supportName).toBe("Maya Chen");
    expect(fm.supportEmail).toBe("maya@rochesterdoulas.com");
    expect(fm.privacyStatement).toBe(PRIVACY_STATEMENT);
  });

  test("pins the URL to the slug and nothing else", () => {
    // The slug is assigned once and never recomputed (00046) because
    // Stripe holds this URL under an ongoing review (#382). Renaming the
    // Practice must not move the page.
    const renamed = frontMatter(renderPage({ ...page, name: "Genesee Birth Collective" }));
    expect(renamed.url).toBe("/p/rochester-doulas/");
    expect(renamed.title).toBe("Genesee Birth Collective");
  });

  test("puts a Practice's own words in front matter, never in the body", () => {
    // The reason the body is empty: markdown would eat the brackets and
    // asterisks out of a real cancellation policy, and raw HTML in it
    // would be one Hugo setting away from being live.
    const hostile: PracticePage = {
      ...page,
      cancellationPolicy: '<script>alert("x")</script> * 50% [refund](javascript:void 0)',
    };
    const rendered = renderPage(hostile);

    expect(rendered.trimEnd().endsWith("}")).toBe(true);
    expect(frontMatter(rendered).cancellationPolicy).toBe(hostile.cancellationPolicy);
  });

  test("escapes a quote rather than breaking the file", () => {
    const quoted = renderPage({ ...page, name: 'The "Best" Doulas: #1' });
    expect(frontMatter(quoted).title).toBe('The "Best" Doulas: #1');
  });

  test("is a leaf bundle, so an image could arrive without the URL moving", () => {
    expect(pageDirectory("rochester-doulas")).toBe("p/rochester-doulas");
  });
});

describe("renderSectionIndex", () => {
  test("suppresses /p/ with the key Hugo still reads", () => {
    // `_build` was the key until Hugo 0.145 removed it, and a removed
    // key is not ignored -- it fails the build. Asserted by name so a
    // rename cannot pass silently and publish an empty directory of
    // every Practice on the platform.
    expect(frontMatter(renderSectionIndex())).toEqual({
      title: "Practices",
      build: { render: "never", list: "never" },
    });
  });
});

describe("toPage", () => {
  const row = {
    slug: "rochester-doulas",
    name: "Rochester Doulas",
    service_description: page.serviceDescription,
    cancellation_policy: page.cancellationPolicy,
    support_name: "Maya Chen",
    support_email: "maya@rochesterdoulas.com",
    published_at: new Date("2026-08-29T14:00:00.000Z"),
  };

  test("reads a complete row", () => {
    expect(toPage(row)).toEqual(page);
  });

  test("refuses a row that would publish an incomplete page", () => {
    // A hosted row without its two facts is impossible (00045's CHECK)
    // and a Practice with no Owner is impossible too -- so this is data
    // corruption, and publishing half a page would hide it behind a
    // Stripe review that then fails for a reason nobody can see.
    expect(() => toPage({ ...row, support_email: null })).toThrow(/support_email/);
    expect(() => toPage({ ...row, cancellation_policy: null })).toThrow(
      /refusing to publish an incomplete page/,
    );
  });

  test("falls back to now when a page has somehow never been published", () => {
    const dated = toPage({ ...row, published_at: null });
    expect(Date.parse(dated.publishedAt)).not.toBeNaN();
  });
});

describe("writePages", () => {
  test("writes one leaf bundle per page and removes what is no longer published", async () => {
    const root = path.join(await mkdtemp(path.join(tmpdir(), "pages-")), "p");

    // A page from an earlier build, for a Practice who has since gone
    // back to her own website. Nothing reconciles it away -- the tree is
    // rebuilt, which is the only way a generated tree forgets.
    await mkdir(path.join(root, "gone-away"), { recursive: true });
    await writeFile(path.join(root, "gone-away", "index.md"), "stale", "utf8");

    await writePages([page], root);

    expect((await readdir(root)).sort()).toEqual(["_index.md", "rochester-doulas"]);
    const written = await readFile(path.join(root, "rochester-doulas", "index.md"), "utf8");
    expect(frontMatter(written).supportEmail).toBe("maya@rochesterdoulas.com");
  });

  test("empties the tree when nothing is published", async () => {
    const root = path.join(await mkdtemp(path.join(tmpdir(), "pages-")), "p");
    await writePages([page], root);
    await writePages([], root);
    // The section index survives -- it is what stops Hugo publishing an
    // empty /p/ -- and no Practice page does.
    expect(await readdir(root)).toEqual(["_index.md"]);
  });
});

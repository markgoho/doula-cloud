/**
 * Regenerates docs/design/doula-cloud.export.md from
 * docs/design/doula-cloud.pen (#1080).
 *
 * Reads the `.pen` file directly off disk as JSON -- no Pen.app, no
 * Pencil MCP server, no `pen` CLI agent session. Those are the working
 * surface for *editing* the design (docs/design/workflow.md); this
 * script only reads what is already committed, which is why a
 * background agent or CI runner can run it with nothing else installed.
 *
 * Usage:
 *   bun run design:export
 *
 * scripts/design-export.test.ts regenerates the export from the real
 * `.pen` file on every `bun test scripts/` and fails, naming this same
 * command, if the committed export is stale.
 */

import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { renderExport, type PenDocument } from "./design-export";

const ROOT = path.resolve(import.meta.dirname, "..");
const PEN_PATH = path.join(ROOT, "docs/design/doula-cloud.pen");
const EXPORT_PATH = path.join(ROOT, "docs/design/doula-cloud.export.md");

async function main(): Promise<void> {
  const raw = await readFile(PEN_PATH, "utf8");
  const document = JSON.parse(raw) as PenDocument;
  const exported = renderExport(document);
  await writeFile(EXPORT_PATH, exported, "utf8");
  console.log(`export-design: wrote ${EXPORT_PATH}`);
}

// Only when run as a program. Importing this file from a test must not
// touch the filesystem.
if (import.meta.main) {
  main().catch((error: unknown) => {
    console.error("export-design:", error);
    process.exit(1);
  });
}

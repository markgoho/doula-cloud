/*
 * The commonly used passwords a Staff password is refused for (#1539).
 *
 * `commonPasswords.txt` is written by `bun run sync-common-passwords`
 * (scripts/sync-common-passwords.ts) and never by hand; docs/environment.md
 * ("The common-password list") says where it comes from. Every entry is
 * lowercased and 15 characters or more.
 *
 * Nothing imports this module statically. `passwordRule.ts` imports it
 * with `import()`, so Vite gives the list a chunk of its own that only the
 * three screens that set a password ever load: about 280 KB of text that
 * every other screen would otherwise carry for nothing.
 */
import raw from './commonPasswords.txt?raw';

// The file ends in a newline; the empty line after it is not an entry.
export const COMMON_PASSWORDS: ReadonlySet<string> = new Set(raw.split('\n').filter((entry) => entry !== ''));

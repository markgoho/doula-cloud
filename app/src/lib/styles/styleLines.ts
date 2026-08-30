/*
 * Walks a Svelte file's `<style>` blocks and returns the lines a source
 * gate should judge, with any block carrying an ignore marker removed.
 *
 * Shared by `tokens.usage.spec.ts` and `layout.usage.spec.ts`, which ask
 * different questions of the same lines and agree exactly on what counts
 * as a line worth asking about: inside a `<style>`, not inside a comment,
 * and not under an in-force exception marker.
 *
 * The marker is passed in rather than fixed, because each gate names its
 * own -- `tokens:ignore`, `layout:ignore` -- so one gate's exception never
 * silences another's.
 */

/*
 * Only what is inside a `<style>` block. Markup and script carry numbers
 * that are genuinely not design values -- an `Icon` size prop, a viewBox,
 * a debounce -- and the design system's claim is about the stylesheet.
 */
export function styleLines(source: string, ignoreMarker: string): { line: number; text: string }[] {
	const lines = source.split('\n');
	const out: { line: number; text: string }[] = [];
	let isInStyle = false;
	let isInBlockComment = false;
	// A pending marker waits for its rule block to open, then stays in force
	// until that block closes again.
	let isMarked = false;
	let markedDepth = 0;
	let depth = 0;
	for (const [index, raw] of lines.entries()) {
		if (/<style[^>]*>/.test(raw)) {
			isInStyle = true;
			continue;
		}
		if (/<\/style>/.test(raw)) {
			isInStyle = false;
			continue;
		}
		if (!isInStyle) continue;

		let text = raw;
		// A comment can open on one line and close on a later one, and most
		// of this repo's rationale is written that way.
		if (isInBlockComment) {
			const close = text.indexOf('*/');
			if (close === -1) {
				if (text.includes(ignoreMarker)) isMarked = true;
				if (isMarked && markedDepth === 0) markedDepth = depth;
				continue;
			}
			text = text.slice(close + 2);
			isInBlockComment = false;
		}
		if (raw.includes(ignoreMarker)) {
			isMarked = true;
			markedDepth = depth;
		}
		text = text.replaceAll(/\/\*.*?\*\//g, '');
		const open = text.indexOf('/*');
		if (open !== -1) {
			isInBlockComment = true;
			if (raw.slice(open).includes(ignoreMarker)) {
				isMarked = true;
				markedDepth = depth;
			}
			text = text.slice(0, open);
		}

		const opened = (text.match(/{/g) ?? []).length;
		const closed = (text.match(/}/g) ?? []).length;
		if (!isMarked && text.trim() !== '') out.push({ line: index + 1, text });
		depth += opened - closed;
		// The marker's block has closed, so the exception stops here rather
		// than leaking down the rest of the stylesheet.
		if (isMarked && opened === 0 && closed > 0 && depth <= markedDepth) {
			isMarked = false;
			markedDepth = 0;
		}
	}
	return out;
}

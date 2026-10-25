/*
 * A Message's attachment, checked on the page before it is sent (#1952).
 *
 * The words are GOV.UK's File upload error messages, and they are the
 * BFF's own (`api/internal/message/create.go`'s `MsgAttachment*` and
 * `MsgMessageOrFile`), so a refusal reads the same whether the page or
 * the server made it. The server stays the real check: it sniffs the
 * file's bytes, which a browser's declared type cannot be trusted for.
 * The page refuses only what it can tell from the file it holds -- its
 * size, and the type the browser or the file's name reports.
 */

// The same 10MB as `maxAttachmentBytes` on the server (10 << 20).
export const MAX_ATTACHMENT_BYTES = 10 * 1024 * 1024;

// What the file input offers in the browser's file picker.
export const ATTACHMENT_ACCEPT = 'image/*,application/pdf';

export const ATTACHMENT_HINT = 'An image or a PDF, up to 10MB';

export const ATTACHMENT_TOO_LARGE = 'The selected file must be smaller than 10MB';
export const ATTACHMENT_EMPTY = 'The selected file is empty';
export const ATTACHMENT_WRONG_TYPE = 'The selected file must be an image or a PDF';
export const MESSAGE_OR_FILE = 'Enter a message or select a file';

/*
 * The extensions read as an image or a PDF when the browser reports no
 * type at all for a file, which it does for a name it does not know.
 */
const ACCEPTED_EXTENSIONS = new Set([
	'avif',
	'bmp',
	'gif',
	'heic',
	'heif',
	'jpeg',
	'jpg',
	'pdf',
	'png',
	'tif',
	'tiff',
	'webp'
]);

function isAcceptedType(file: File): boolean {
	if (file.type !== '') return file.type === 'application/pdf' || file.type.startsWith('image/');
	const dot = file.name.lastIndexOf('.');
	// No extension and no type: nothing the page can tell, so the
	// server's sniff decides.
	if (dot === -1) return true;
	return ACCEPTED_EXTENSIONS.has(file.name.slice(dot + 1).toLowerCase());
}

/*
 * Why the page refuses `file`, or undefined when it can find no reason.
 * Empty first, because an empty file has no type to be wrong about.
 */
export function attachmentError(file: File): string | undefined {
	if (file.size === 0) return ATTACHMENT_EMPTY;
	if (file.size > MAX_ATTACHMENT_BYTES) return ATTACHMENT_TOO_LARGE;
	if (!isAcceptedType(file)) return ATTACHMENT_WRONG_TYPE;
	return undefined;
}

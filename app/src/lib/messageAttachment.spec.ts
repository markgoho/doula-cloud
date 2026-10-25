import { describe, expect, it } from 'vitest';
import {
	ATTACHMENT_EMPTY,
	ATTACHMENT_TOO_LARGE,
	ATTACHMENT_WRONG_TYPE,
	attachmentError,
	MAX_ATTACHMENT_BYTES
} from './messageAttachment.js';

function fileOf(size: number, name: string, type: string): File {
	return new File([new Uint8Array(size)], name, { type });
}

describe('attachmentError', () => {
	it('accepts an image', () => {
		expect(attachmentError(fileOf(4, 'photo.png', 'image/png'))).toBeUndefined();
	});

	it('accepts a PDF', () => {
		expect(attachmentError(fileOf(4, 'form.pdf', 'application/pdf'))).toBeUndefined();
	});

	it('accepts a file of exactly 10MB', () => {
		expect(attachmentError(fileOf(MAX_ATTACHMENT_BYTES, 'scan.jpg', 'image/jpeg'))).toBeUndefined();
	});

	it('refuses a file over 10MB', () => {
		expect(attachmentError(fileOf(MAX_ATTACHMENT_BYTES + 1, 'scan.jpg', 'image/jpeg'))).toBe(
			ATTACHMENT_TOO_LARGE
		);
	});

	it('refuses an empty file as empty, not as the wrong type', () => {
		expect(attachmentError(fileOf(0, 'notes.txt', 'text/plain'))).toBe(ATTACHMENT_EMPTY);
	});

	it('refuses a type that is neither an image nor a PDF', () => {
		expect(attachmentError(fileOf(4, 'notes.txt', 'text/plain'))).toBe(ATTACHMENT_WRONG_TYPE);
	});

	it('reads the extension when the browser reports no type', () => {
		expect(attachmentError(fileOf(4, 'scan.HEIC', ''))).toBeUndefined();
		expect(attachmentError(fileOf(4, 'notes.docx', ''))).toBe(ATTACHMENT_WRONG_TYPE);
	});

	it('leaves a file with no type and no extension to the server', () => {
		expect(attachmentError(fileOf(4, 'scan', ''))).toBeUndefined();
	});
});

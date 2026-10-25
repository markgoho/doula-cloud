import { describe, expect, it } from 'vitest';
import {
	ATTACHMENT_ACCEPT,
	ATTACHMENT_EMPTY,
	ATTACHMENT_HINT,
	ATTACHMENT_TOO_LARGE,
	ATTACHMENT_WRONG_TYPE,
	attachmentError,
	MAX_ATTACHMENT_BYTES
} from './messageAttachment.js';

function fileOf(size: number, name: string, type: string): File {
	return new File([new Uint8Array(size)], name, { type });
}

describe('attachmentError', () => {
	// The list the server's sniff accepts (`api/internal/message/create.go`,
	// #1977), and the words that name it.
	it.each([
		['photo.jpg', 'image/jpeg'],
		['photo.png', 'image/png'],
		['photo.gif', 'image/gif'],
		['photo.webp', 'image/webp'],
		['form.pdf', 'application/pdf']
	])('accepts %s', (name, type) => {
		expect(attachmentError(fileOf(4, name, type))).toBeUndefined();
	});

	it('names the accepted types in the hint, the wrong-type message and the picker', () => {
		expect(ATTACHMENT_HINT).toBe('A JPG, PNG, GIF, WEBP or PDF, up to 10MB');
		expect(ATTACHMENT_WRONG_TYPE).toBe('The selected file must be a JPG, PNG, GIF, WEBP or PDF');
		// JPEG first: iOS Safari converts a HEIC photo to the first listed
		// type when the list leaves HEIC out (WebKit bugs 267277, 292350).
		expect(ATTACHMENT_ACCEPT).toBe('image/jpeg,image/png,image/gif,image/webp,application/pdf');
	});

	// #1977: an iPhone photo the browser hands over as HEIC is refused
	// before sending, in the words that name the accepted types, since the
	// server's sniff cannot read it and most browsers cannot show it.
	it.each([
		['an iPhone HEIC photo', 'IMG_0001.HEIC', 'image/heic'],
		['a HEIF image', 'IMG_0001.heif', 'image/heif'],
		['a BMP image', 'scan.bmp', 'image/bmp'],
		['an SVG image', 'logo.svg', 'image/svg+xml']
	])('refuses %s as the wrong type', (_name, name, type) => {
		expect(attachmentError(fileOf(4, name, type))).toBe(ATTACHMENT_WRONG_TYPE);
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
		expect(attachmentError(fileOf(4, 'photo.JPEG', ''))).toBeUndefined();
		expect(attachmentError(fileOf(4, 'scan.HEIC', ''))).toBe(ATTACHMENT_WRONG_TYPE);
		expect(attachmentError(fileOf(4, 'notes.docx', ''))).toBe(ATTACHMENT_WRONG_TYPE);
	});

	it('leaves a file with no type and no extension to the server', () => {
		expect(attachmentError(fileOf(4, 'scan', ''))).toBeUndefined();
	});
});

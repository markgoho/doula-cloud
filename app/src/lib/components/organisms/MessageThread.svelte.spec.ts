import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { formatInstantWithClock } from '../../dates.js';
import { RefusalError, SERVICE_PROBLEM } from '../../formErrors.js';
import {
	ATTACHMENT_ACCEPT,
	ATTACHMENT_EMPTY,
	ATTACHMENT_HINT,
	ATTACHMENT_TOO_LARGE,
	ATTACHMENT_WRONG_TYPE,
	MAX_ATTACHMENT_BYTES,
	MESSAGE_OR_FILE
} from '../../messageAttachment.js';
import MessageThread, { type Message } from './MessageThread.svelte';

const messages: Message[] = [
	{
		messageId: 'm1',
		senderType: 'staff',
		senderId: 's1',
		senderName: 'Ada Lovelace',
		body: 'Hello there',
		createdAt: '2026-01-01T12:00:00Z'
	},
	{
		messageId: 'm2',
		senderType: 'client',
		senderId: 'c1',
		senderName: 'Grace Hopper',
		body: 'A photo for you',
		attachmentFilename: 'photo.png',
		attachmentContentType: 'image/png',
		createdAt: '2026-01-02T12:00:00Z'
	}
];

interface SetupOptions {
	messages?: Message[];
	error?: string;
	hasMore?: boolean;
	isLoadingOlder?: boolean;
	onSend?: (body: string, attachment: File | undefined) => Promise<void>;
	attachmentPreviewURLs?: Record<string, string>;
	readerSenderType?: Message['senderType'];
	composeOpener?: string;
	cannotSendReason?: string;
}

async function setup({
	messages: messagesOption = messages,
	error = '',
	hasMore = false,
	isLoadingOlder = false,
	onSend = vi.fn().mockResolvedValue(undefined),
	attachmentPreviewURLs = {},
	readerSenderType,
	composeOpener,
	cannotSendReason
}: SetupOptions = {}) {
	const onLoadOlder = vi.fn();
	const onDownloadAttachment = vi.fn();
	const { container } = await render(MessageThread, {
		messages: messagesOption,
		error,
		hasMore,
		isLoadingOlder,
		onLoadOlder,
		onSend,
		onDownloadAttachment,
		attachmentPreviewURLs,
		readerSenderType,
		composeOpener,
		cannotSendReason
	});
	return { container, onLoadOlder, onSend, onDownloadAttachment };
}

describe('MessageThread.svelte', () => {
	it('renders each message with sender, type, timestamp, and body', async () => {
		await setup();

		await expect.element(page.getByText('Ada Lovelace')).toBeVisible();
		await expect.element(page.getByText('Hello there')).toBeVisible();
		await expect.element(page.getByText('Grace Hopper')).toBeVisible();
		await expect.element(page.getByText('A photo for you')).toBeVisible();
	});

	// A message's createdAt is an instant (message/list.go), read in the
	// reader's zone month first, to the minute (#1958).
	it('prints when each message was sent through dates.ts, month first', async () => {
		await setup();

		await expect.element(page.getByText(formatInstantWithClock(messages[0]!.createdAt))).toBeVisible();
		await expect.element(page.getByText(/^Jan 1, 2026, \d{1,2}:00(am|pm)$/)).toBeVisible();
	});

	// #2023: a message is labeled by who sent it, never by a role in
	// brackets, and the reader's own messages read "You".
	it('names each sender with no role, and the reader as "You"', async () => {
		const { container } = await setup({ readerSenderType: 'client' });

		await expect.element(page.getByText('Ada Lovelace')).toBeVisible();
		await expect.element(page.getByText('You', { exact: true })).toBeVisible();
		expect(page.getByText('Grace Hopper').elements()).toHaveLength(0);
		expect(container.textContent).not.toMatch(/\((client|staff)\)/);
	});

	it('omits the body paragraph for an attachment-only message with no body text', async () => {
		const { container } = await setup({
			messages: [{ ...messages[1]!, body: '' }]
		});

		expect(container.querySelector(':scope li p')).not.toBeInTheDocument();
	});

	it('renders "No messages yet." when messages is empty', async () => {
		await setup({ messages: [] });

		await expect.element(page.getByText('No messages yet.')).toBeVisible();
	});

	it('renders the error as a Notice when set', async () => {
		await setup({ error: 'Failed to load messages' });

		await expect.element(page.getByRole('alert')).toHaveTextContent('Failed to load messages');
	});

	it('renders no load-older button when hasMore is false', async () => {
		await setup();

		await expect.element(page.getByRole('button', { name: 'Load older messages' })).not.toBeInTheDocument();
	});

	it('renders a load-older button and calls onLoadOlder when clicked', async () => {
		const { onLoadOlder } = await setup({ hasMore: true });

		await page.getByRole('button', { name: 'Load older messages' }).click();

		expect(onLoadOlder).toHaveBeenCalledOnce();
	});

	it('renders an attachment download button and calls onDownloadAttachment with the messageId and filename', async () => {
		const { onDownloadAttachment } = await setup();

		await page.getByRole('button', { name: 'photo.png' }).click();

		expect(onDownloadAttachment).toHaveBeenCalledWith('m2', 'photo.png');
	});

	it('renders an inline preview image when attachmentPreviewURLs has an entry for the message', async () => {
		const { container } = await setup({ attachmentPreviewURLs: { m2: 'blob:preview-url' } });

		const image = container.querySelector('img.attachment-preview');
		expect(image).toHaveAttribute('src', 'blob:preview-url');
		expect(image).toHaveAttribute('alt', 'photo.png');
	});

	it('renders no preview image when attachmentPreviewURLs has no entry for the message', async () => {
		const { container } = await setup();

		expect(container.querySelector('img.attachment-preview')).not.toBeInTheDocument();
	});

});

const messageField = () => page.getByLabelText('Message');
const attachmentField = () => page.getByLabelText('Attachment');
const send = () => page.getByRole('button', { name: 'Send' }).click();
const summary = () => page.getByRole('alert').filter({ hasText: 'There is a problem' });

function fileOf(size: number, name: string, type: string): File {
	return new File([new Uint8Array(size)], name, { type });
}

describe('MessageThread.svelte compose form', () => {
	it('labels the attachment and states its types and limit in a separate hint', async () => {
		await setup();

		await expect.element(attachmentField()).toBeVisible();
		await expect.element(attachmentField()).toHaveAccessibleDescription(ATTACHMENT_HINT);
		await expect.element(attachmentField()).not.toHaveAttribute('aria-invalid');
		// The picker offers the same list the hint names (#1977).
		await expect.element(attachmentField()).toHaveAttribute('accept', ATTACHMENT_ACCEPT);
	});

	it('calls onSend with the typed body and selected attachment on submit', async () => {
		const { onSend } = await setup();

		await messageField().fill('New message');
		const file = fileOf(4, 'notes.pdf', 'application/pdf');
		await attachmentField().upload(file);
		await send();

		expect(onSend).toHaveBeenCalledWith('New message', file);
	});

	it('clears the message and the file after a successful send', async () => {
		const { onSend } = await setup();

		await messageField().fill('New message');
		await attachmentField().upload(fileOf(4, 'photo.png', 'image/png'));
		await send();
		await expect.element(messageField()).toHaveValue('');
		await expect.element(attachmentField()).toHaveValue('');

		// The cleared file is not sent again with the next message.
		await messageField().fill('Second message');
		await send();
		expect(onSend).toHaveBeenLastCalledWith('Second message', undefined);
	});

	it('marks the send button busy while a send is in flight', async () => {
		await setup({ onSend: () => new Promise<void>(() => {}) });

		await messageField().fill('New message');
		await send();

		await expect.element(page.getByRole('button', { name: 'Send' })).toBeDisabled();
	});

	it.each([
		['a file over 10MB', fileOf(MAX_ATTACHMENT_BYTES + 1, 'scan.jpg', 'image/jpeg'), ATTACHMENT_TOO_LARGE],
		['an empty file', fileOf(0, 'photo.png', 'image/png'), ATTACHMENT_EMPTY],
		['a file that is neither an image nor a PDF', fileOf(4, 'notes.txt', 'text/plain'), ATTACHMENT_WRONG_TYPE],
		['an iPhone HEIC photo', fileOf(4, 'IMG_0001.HEIC', 'image/heic'), ATTACHMENT_WRONG_TYPE]
	])('refuses %s against the attachment before sending', async (_name, file, words) => {
		const { onSend } = await setup();

		await attachmentField().upload(file);
		await send();

		expect(onSend).not.toHaveBeenCalled();
		await expect.element(summary().getByRole('link', { name: words })).toBeVisible();
		await expect.element(attachmentField()).toHaveAttribute('aria-invalid', 'true');
		await expect.element(attachmentField()).toHaveAccessibleDescription(`${ATTACHMENT_HINT} ${words}`);
	});

	it('refuses an empty compose form against the message before sending', async () => {
		const { onSend } = await setup();

		await send();

		expect(onSend).not.toHaveBeenCalled();
		await expect.element(summary().getByRole('link', { name: MESSAGE_OR_FILE })).toBeVisible();
		await expect.element(messageField()).toHaveAccessibleDescription(MESSAGE_OR_FILE);
	});

	it('shows a refusal the server keys to the attachment against the attachment, and keeps the draft', async () => {
		const onSend = vi.fn().mockRejectedValue(
			new RefusalError(ATTACHMENT_WRONG_TYPE, { attachment: ATTACHMENT_WRONG_TYPE })
		);
		await setup({ onSend });

		await messageField().fill('New message');
		await attachmentField().upload(fileOf(4, 'photo.png', 'image/png'));
		await send();

		await expect.element(summary().getByRole('link', { name: ATTACHMENT_WRONG_TYPE })).toBeVisible();
		await expect
			.element(attachmentField())
			.toHaveAccessibleDescription(`${ATTACHMENT_HINT} ${ATTACHMENT_WRONG_TYPE}`);
		await expect.element(messageField()).toHaveValue('New message');
	});

	it('shows a refusal the server keys to the message against the message', async () => {
		const onSend = vi.fn().mockRejectedValue(new RefusalError(MESSAGE_OR_FILE, { body: MESSAGE_OR_FILE }));
		await setup({ onSend });

		await messageField().fill('Hello');
		await send();

		expect(onSend).toHaveBeenCalledOnce();
		await expect.element(messageField()).toHaveAccessibleDescription(MESSAGE_OR_FILE);
	});

	it('shows a refusal that names no field in the summary, with no link', async () => {
		const onSend = vi.fn().mockRejectedValue(new Error(SERVICE_PROBLEM));
		await setup({ onSend });

		await messageField().fill('New message');
		await send();

		await expect.element(summary().getByText(SERVICE_PROBLEM)).toBeVisible();
		await expect.element(summary().getByRole('link')).not.toBeInTheDocument();
		await expect.element(messageField()).toHaveValue('New message');
	});
});

// #2004: on a page where the thread is one section among many, the form
// opens only when asked, and a send closes it with a focused status.
describe('MessageThread.svelte compose behind an opener (#2004)', () => {
	it('reads the thread with no form until the opener is pressed', async () => {
		await setup({ composeOpener: 'Write a message' });

		expect(page.getByLabelText('Message').elements()).toHaveLength(0);
		await page.getByRole('button', { name: 'Write a message' }).click();

		await expect.element(page.getByRole('heading', { level: 3, name: 'Write a message' })).toHaveFocus();
		await expect.element(page.getByLabelText('Message')).toBeVisible();
	});

	it('closes the form on Cancel, with its refusals gone', async () => {
		await setup({ composeOpener: 'Write a message' });

		await page.getByRole('button', { name: 'Write a message' }).click();
		await page.getByRole('button', { name: 'Send', exact: true }).click();
		await expect.element(page.getByText('There is a problem')).toBeVisible();
		await page.getByRole('button', { name: 'Cancel' }).click();

		expect(page.getByLabelText('Message').elements()).toHaveLength(0);
		expect(page.getByText('There is a problem').elements()).toHaveLength(0);
		// #2103: back on the opener, not on the page.
		await expect.element(page.getByRole('button', { name: 'Write a message' })).toHaveFocus();
	});

	it('closes the form once a message is away, and says so, focused', async () => {
		const { onSend } = await setup({ composeOpener: 'Write a message' });

		await page.getByRole('button', { name: 'Write a message' }).click();
		await page.getByLabelText('Message').fill('See you Thursday.');
		await page.getByRole('button', { name: 'Send', exact: true }).click();

		expect(onSend).toHaveBeenCalledWith('See you Thursday.', undefined);
		await expect.element(page.getByRole('status').filter({ hasText: 'Message sent.' })).toHaveFocus();
		expect(page.getByLabelText('Message').elements()).toHaveLength(0);
	});

	it('offers no form at all where no message can be sent, and says why', async () => {
		await setup({ composeOpener: 'Write a message', cannotSendReason: 'The thread opens once Grace has accepted.' });

		await expect.element(page.getByText('The thread opens once Grace has accepted.')).toBeVisible();
		expect(page.getByRole('button', { name: 'Write a message' }).elements()).toHaveLength(0);
		expect(page.getByLabelText('Message').elements()).toHaveLength(0);
	});
});

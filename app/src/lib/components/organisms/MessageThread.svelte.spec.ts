import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
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
	isSending?: boolean;
	onSend?: (body: string, attachment: File | undefined) => Promise<boolean>;
	attachmentPreviewURLs?: Record<string, string>;
}

async function setup({
	messages: messagesOption = messages,
	error = '',
	hasMore = false,
	isLoadingOlder = false,
	isSending = false,
	onSend = vi.fn().mockResolvedValue(true),
	attachmentPreviewURLs = {}
}: SetupOptions = {}) {
	const onLoadOlder = vi.fn();
	const onDownloadAttachment = vi.fn();
	const { container } = await render(MessageThread, {
		messages: messagesOption,
		error,
		hasMore,
		isLoadingOlder,
		isSending,
		onLoadOlder,
		onSend,
		onDownloadAttachment,
		attachmentPreviewURLs
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

	it('calls onSend with the typed body and selected attachment on submit', async () => {
		const onSend = vi.fn().mockResolvedValue(true);
		await setup({ onSend });

		await page.getByLabelText('Message').fill('New message');
		const file = new File(['data'], 'notes.pdf', { type: 'application/pdf' });
		await page.getByLabelText('Attachment (image or PDF, up to 10MB)').upload(file);
		await page.getByRole('button', { name: 'Send' }).click();

		expect(onSend).toHaveBeenCalledWith('New message', file);
	});

	it('clears the compose form after a successful send', async () => {
		const onSend = vi.fn().mockResolvedValue(true);
		await setup({ onSend });

		await page.getByLabelText('Message').fill('New message');
		await page.getByRole('button', { name: 'Send' }).click();

		await expect.element(page.getByLabelText('Message')).toHaveValue('');
	});

	it('keeps the compose form after a failed send', async () => {
		const onSend = vi.fn().mockResolvedValue(false);
		await setup({ onSend });

		await page.getByLabelText('Message').fill('New message');
		await page.getByRole('button', { name: 'Send' }).click();

		await expect.element(page.getByLabelText('Message')).toHaveValue('New message');
	});

	it('marks the send button busy when isSending is true', async () => {
		await setup({ isSending: true });

		await expect.element(page.getByRole('button', { name: 'Send' })).toBeDisabled();
	});
});

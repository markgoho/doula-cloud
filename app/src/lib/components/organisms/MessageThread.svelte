<script module lang="ts">
	export interface Message {
		messageId: string;
		senderType: 'staff' | 'client';
		senderId: string;
		senderName: string;
		body: string;
		attachmentFilename?: string;
		attachmentContentType?: string;
		createdAt: string;
	}
</script>

<script lang="ts">
	import Button from '../atoms/Button.svelte';
	import Notice from '../atoms/Notice.svelte';

	interface Properties {
		messages: Message[];
		error: string;
		hasMore: boolean;
		isLoadingOlder: boolean;
		isSending: boolean;
		onLoadOlder: () => void;
		onSend: (body: string, attachment: File | undefined) => Promise<boolean>;
		onDownloadAttachment: (messageId: string, filename: string) => void;
		attachmentPreviewURLs: Record<string, string>;
	}

	let {
		messages,
		error,
		hasMore,
		isLoadingOlder,
		isSending,
		onLoadOlder,
		onSend,
		onDownloadAttachment,
		attachmentPreviewURLs
	}: Properties = $props();

	let body = $state('');
	let attachment = $state<File | undefined>();

	// onSend reports whether the send succeeded so the compose form only
	// clears on success -- a failed send (e.g. network error) leaves the
	// draft in place for the person to retry, matching the pages' prior
	// inline behavior.
	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		const didSend = await onSend(body, attachment);
		if (didSend) {
			body = '';
			attachment = undefined;
		}
	}
</script>

<stack-l>
	{#if error}
		<Notice variant="error" message={error} />
	{/if}

	{#if hasMore}
		<Button label="Load older messages" variant="secondary" loading={isLoadingOlder} onClick={onLoadOlder} />
	{/if}

	{#if messages.length === 0}
		<p>No messages yet.</p>
	{:else}
		<ul>
			{#each messages as message (message.messageId)}
				<li>
					<stack-l space="var(--space-1)">
						<cluster-l space="var(--space-2)" align="baseline">
							<strong>{message.senderName}</strong>
							<!-- v8 ignore start: Svelte's compiled null-guard on these text nodes is unreachable -- senderType is a required union member and toLocaleString always returns a string -->
							<span class="meta">({message.senderType}) — {new Date(message.createdAt).toLocaleString()}</span>
							<!-- v8 ignore stop -->
						</cluster-l>
						{#if message.body}
							<p>{message.body}</p>
						{/if}
						{#if message.attachmentFilename}
							{@const filename = message.attachmentFilename}
							{#if attachmentPreviewURLs[message.messageId]}
								<img
									class="attachment-preview"
									src={attachmentPreviewURLs[message.messageId]}
									alt={filename}
								/>
							{/if}
							<Button
								label={filename}
								icon="paperclip"
								variant="secondary"
								size="sm"
								onClick={() => onDownloadAttachment(message.messageId, filename)}
							/>
						{/if}
					</stack-l>
				</li>
			{/each}
		</ul>
	{/if}

	<form onsubmit={handleSubmit}>
		<stack-l space="var(--space-2)">
			<label>
				Message
				<textarea bind:value={body}></textarea>
			</label>
			<label>
				Attachment (image or PDF, up to 10MB)
				<input
					type="file"
					accept="image/*,application/pdf"
					onchange={(event) => (attachment = event.currentTarget.files?.[0])}
				/>
			</label>
			<Button type="submit" label="Send" loading={isSending} />
		</stack-l>
	</form>
</stack-l>

<style>
	@layer components {
		ul {
			list-style: none;
			margin: 0;
			padding: 0;
		}

		li {
			padding-block: var(--space-3);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
		}

		.meta {
			color: var(--color-on-surface-variant);
			font-size: var(--text-body-sm-size);
		}

		.attachment-preview {
			display: block;
			max-inline-size: 240px;
			max-block-size: 240px;
			border-radius: var(--radius);
		}

		label {
			display: block;
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface);
		}

		textarea {
			display: block;
			inline-size: 100%;
			min-block-size: 4rem;
			margin-block-start: var(--space-1);
			padding: var(--space-2) var(--space-3);
			font-family: var(--font-family-base);
			color: var(--color-on-surface);
			background-color: var(--color-surface);
			border: var(--border-thin) solid var(--color-outline);
			border-radius: var(--radius);
		}

		textarea:focus-visible {
			outline: 2px solid var(--color-primary);
			outline-offset: 2px;
		}

		input[type='file'] {
			display: block;
			margin-block-start: var(--space-1);
		}
	}
</style>

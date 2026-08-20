<script lang="ts">
	import MessageThread, { type Message } from '#lib/components/organisms/MessageThread.svelte';

	const messages: Message[] = [
		{
			messageId: 'm1',
			senderType: 'staff',
			senderId: 's1',
			senderName: 'Ada Lovelace',
			body: 'Looking forward to our next visit!',
			createdAt: '2026-01-01T12:00:00Z'
		},
		{
			messageId: 'm2',
			senderType: 'client',
			senderId: 'c1',
			senderName: 'Grace Hopper',
			body: 'Here is the photo you asked for.',
			attachmentFilename: 'ultrasound.png',
			attachmentContentType: 'image/png',
			createdAt: '2026-01-02T12:00:00Z'
		}
	];

	async function didSend() {
		return true;
	}

	function onLoadOlder() {}

	function onDownloadAttachment() {}

	const previewImage =
		'data:image/svg+xml,' +
		encodeURIComponent(
			'<svg xmlns="http://www.w3.org/2000/svg" width="240" height="160"><rect width="240" height="160" fill="#cbb7d6"/></svg>'
		);
</script>

<stack-l space="var(--space-6)">
	<h1>Message thread</h1>

	<section>
		<h2>Default</h2>
		<MessageThread
			{messages}
			error=""
			hasMore={false}
			isLoadingOlder={false}
			isSending={false}
			{onLoadOlder}
			onSend={didSend}
			{onDownloadAttachment}
			attachmentPreviewURLs={{}}
		/>
	</section>

	<section>
		<h2>With an inline image preview</h2>
		<MessageThread
			{messages}
			error=""
			hasMore={false}
			isLoadingOlder={false}
			isSending={false}
			{onLoadOlder}
			onSend={didSend}
			{onDownloadAttachment}
			attachmentPreviewURLs={{ m2: previewImage }}
		/>
	</section>

	<section>
		<h2>Load older messages</h2>
		<MessageThread
			{messages}
			error=""
			hasMore={true}
			isLoadingOlder={false}
			isSending={false}
			{onLoadOlder}
			onSend={didSend}
			{onDownloadAttachment}
			attachmentPreviewURLs={{}}
		/>
	</section>

	<section>
		<h2>Error</h2>
		<MessageThread
			{messages}
			error="Failed to load messages"
			hasMore={false}
			isLoadingOlder={false}
			isSending={false}
			{onLoadOlder}
			onSend={didSend}
			{onDownloadAttachment}
			attachmentPreviewURLs={{}}
		/>
	</section>

	<section>
		<h2>Sending</h2>
		<MessageThread
			{messages}
			error=""
			hasMore={false}
			isLoadingOlder={false}
			isSending={true}
			{onLoadOlder}
			onSend={didSend}
			{onDownloadAttachment}
			attachmentPreviewURLs={{}}
		/>
	</section>

	<section>
		<h2>Empty</h2>
		<MessageThread
			messages={[]}
			error=""
			hasMore={false}
			isLoadingOlder={false}
			isSending={false}
			{onLoadOlder}
			onSend={didSend}
			{onDownloadAttachment}
			attachmentPreviewURLs={{}}
		/>
	</section>
</stack-l>

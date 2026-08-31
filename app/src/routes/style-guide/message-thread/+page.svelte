<script lang="ts">
	import MessageThread, { type Message } from '#lib/components/organisms/MessageThread.svelte';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): a
	 * message is free text, so it is where a Practice pastes a link -- the
	 * one value a browser will not break (#521) -- and an attachment is
	 * named by whatever the phone that took it called the file.
	 */
	const messages: Message[] = [
		{
			messageId: 'm1',
			senderType: 'staff',
			senderId: 's1',
			senderName: 'Persephone Adeyemi-Wollstonecraft',
			body:
				'Looking forward to our next visit! Your birth plan is at https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake -- have a read before Thursday.',
			createdAt: '2026-01-01T12:00:00Z'
		},
		{
			messageId: 'm2',
			senderType: 'client',
			senderId: 'c1',
			senderName: 'Anne-Marie Ochieng-Whitfield',
			body: 'Here is the photo you asked for, taken at the appointment this morning.',
			attachmentFilename: 'ultrasound-scan-anne-marie-ochieng-whitfield-2027-09-14.png',
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
			error="We could not load the rest of this conversation. Check your connection and try again."
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

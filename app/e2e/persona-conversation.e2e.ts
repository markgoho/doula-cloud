// Exercises AnthropicPersonaConversation against a fake MessagesClient --
// #821 forbids a spec from calling a live model, so nothing here ever
// reaches the network. Named *.e2e.ts, not a Vitest spec, because
// vite.config.ts's unit-test projects only include src/** (app/e2e is
// exercised by the Playwright suite instead, per docs/testing.md).
import { test, expect } from '@playwright/test';
import type Anthropic from '@anthropic-ai/sdk';
import { AnthropicPersonaConversation, type MessagesClient } from './simulation/persona-conversation';

// Always answers with a fixed judgment and captures every request it was
// sent, so a test can inspect exactly what history the conversation built
// up without any real API call.
class FakeMessagesClient implements MessagesClient {
	messages: MessagesClient['messages'];
	readonly requests: Anthropic.MessageCreateParamsNonStreaming[] = [];

	constructor(private readonly reply: () => string) {
		this.messages = {
			create: async (parameters: Anthropic.MessageCreateParamsNonStreaming): Promise<Anthropic.Message> => {
				this.requests.push(parameters);
				return { content: [{ type: 'text', text: this.reply(), citations: undefined }] } as unknown as Anthropic.Message;
			}
		};
	}
}

test('judge() sends the compact act record and parses the four-outcome judgment', async () => {
	const client = new FakeMessagesClient(() => JSON.stringify({ outcome: 'completed with friction', narrated: 'I had to guess.' }));
	const conversation = new AnthropicPersonaConversation('practice-owner', { client, personaBrief: 'You are Renata Alvarez.' });

	const judgment = await conversation.judge({ id: '2.2', act: 'Submitted the invite form', result: 'POST /invitations -> 201 created' });

	expect(judgment).toEqual({ outcome: 'completed with friction', narrated: 'I had to guess.' });
	expect(client.requests).toHaveLength(1);
	const sent = JSON.stringify(client.requests[0].messages);
	expect(sent).toContain('Submitted the invite form');
	expect(sent).toContain('POST /invitations -> 201 created');
});

test('judge() tolerates a markdown-fenced JSON reply', async () => {
	const client = new FakeMessagesClient(() => '```json\n{"outcome": "refused", "narrated": "It just said no."}\n```');
	const conversation = new AnthropicPersonaConversation('practice-owner', { client, personaBrief: 'You are Renata Alvarez.' });

	const judgment = await conversation.judge({ id: '2.2-a', act: 'Submitted it again', result: '409 conflict' });

	expect(judgment).toEqual({ outcome: 'refused', narrated: 'It just said no.' });
});

test('judge() refuses to invent an outcome the format does not have', async () => {
	const client = new FakeMessagesClient(() => JSON.stringify({ outcome: 'confused', narrated: 'hmm' }));
	const conversation = new AnthropicPersonaConversation('practice-owner', { client, personaBrief: 'You are Renata Alvarez.' });

	await expect(conversation.judge({ id: '1', act: 'a', result: 'r' })).rejects.toThrow(/outcome/);
});

test('narrateWait() returns a plain-text line, not JSON', async () => {
	const client = new FakeMessagesClient(() => '  That took a while for nothing on screen.  ');
	const conversation = new AnthropicPersonaConversation('practice-owner', { client, personaBrief: 'You are Renata Alvarez.' });

	const line = await conversation.narrateWait(3910);

	expect(line).toBe('That took a while for nothing on screen.');
	// requests[0].messages is the same array the conversation keeps
	// appending to, so by the time the call resolves it already carries
	// the assistant reply too -- the user prompt that was actually sent
	// is the one just before it.
	expect(client.requests[0].messages.at(-2)).toMatchObject({ role: 'user' });
	expect(JSON.stringify(client.requests[0].messages)).toContain('3910');
});

test('record() appends to history without ever calling the client', async () => {
	const client = new FakeMessagesClient(() => JSON.stringify({ outcome: 'completed' }));
	const conversation = new AnthropicPersonaConversation('practice-owner', { client, personaBrief: 'You are Renata Alvarez.' });

	conversation.record({ id: '1.1', act: 'Opened the roster', result: 'The roster rendered.', outcome: 'completed' });
	expect(client.requests).toHaveLength(0);

	// The next judge() call resends the full history, so the recorded
	// entry is now part of what she carries into her next judgment.
	await conversation.judge({ id: '1.2', act: 'Opened a Client', result: 'The Client rendered.' });
	const sent = JSON.stringify(client.requests[0].messages);
	expect(sent).toContain('Opened the roster');
});

test('her held history never carries an image or a page snapshot, and fits a full run for the heaviest persona', async () => {
	const client = new FakeMessagesClient(() => JSON.stringify({ outcome: 'completed' }));
	const conversation = new AnthropicPersonaConversation('practice-owner', { client, personaBrief: 'You are Renata Alvarez, and you carry more Clients than anyone.' });

	// calendar.md sizes a full run at ~1,526 acts across 22 cast members,
	// with Renata as the heaviest single persona -- 500 of her own acts is
	// a deliberately generous stand-in for that.
	const ACTS = 500;
	for (let index = 0; index < ACTS; index++) {
		const judgment = await conversation.judge({ id: `${index}`, act: `Act ${index}`, result: `Result ${index}` });
		conversation.record({ id: `${index}`, act: `Act ${index}`, result: `Result ${index}`, outcome: judgment.outcome, narrated: judgment.narrated });
	}

	const lastRequest = client.requests.at(-1);
	expect(lastRequest).toBeDefined();
	const messages = lastRequest?.messages ?? [];

	for (const message of messages) {
		// Every message this class ever builds is a bare string -- never a
		// content-block array, so an image block is structurally
		// impossible, not just absent by luck.
		expect(typeof message.content).toBe('string');
	}

	const totalChars = messages.reduce((sum, message) => sum + (typeof message.content === 'string' ? message.content.length : 0), 0);
	// 1,000,000 characters is roughly 250K tokens -- comfortably inside
	// Claude's 1M-token window with headroom to spare, for the heaviest
	// persona at the end of a full run.
	expect(totalChars).toBeLessThan(1_000_000);
});

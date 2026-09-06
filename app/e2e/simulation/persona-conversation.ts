// A Persona is an LLM turn (#821): one stateful, append-only Anthropic
// Messages API conversation held for the whole run, asked only what a
// script cannot know -- whether the product refused her or she got
// stuck, and the Narrated line she owes when it did. It never holds the
// Page, never decides a timing, and never sees a screenshot or a page
// snapshot -- only the compact per-act record entry.ts already defines:
// id, act, result, outcome, narrated.
import type Anthropic from '@anthropic-ai/sdk';
import type { Outcome } from './entry';

export interface PersonaTurnRecord {
	id: string;
	act: string;
	result: string;
}

export interface PersonaJudgment {
	outcome: Outcome;
	narrated?: string;
}

// What actually got logged for one of her acts, after capture.ts applied
// the performance bands. This is what record() appends to her history --
// the same shape the log itself holds, so her conversation and the log
// never drift apart.
export interface PersonaTurnResult extends PersonaTurnRecord {
	outcome: Outcome;
	narrated?: string;
}

export interface PersonaConversation {
	readonly slug: string;
	judge(record: PersonaTurnRecord): Promise<PersonaJudgment>;
	// capture.ts's narrateWait hook: the timing band alone pushed a
	// 'completed' act into friction, and this is the one line she owes it.
	narrateWait(timingMs: number): Promise<string>;
	// Appends one act's final, banded outcome to her held history. Called
	// once per act no matter which method above produced the judgment.
	record(result: PersonaTurnResult): void;
}

// The minimal surface this module needs from the Anthropic SDK client --
// deliberately narrower than the SDK's own class so a test can inject a
// fake without constructing a real Anthropic() or touching the network.
export interface MessagesClient {
	messages: {
		create(parameters: Anthropic.MessageCreateParamsNonStreaming): Promise<Anthropic.Message>;
	};
}

const OUTCOMES: ReadonlySet<Outcome> = new Set(['completed', 'completed with friction', 'refused', 'stuck']);
const DEFAULT_MODEL = 'claude-opus-5';

export interface AnthropicPersonaConversationOptions {
	client: MessagesClient;
	// Read from docs/personas/<slug>.md by the caller -- this module never
	// touches the filesystem, so a rehearsal or a test can hand it any
	// brief without a real persona file existing.
	personaBrief: string;
	model?: string;
}

// Holds one persona's whole-run conversation. The history is a plain,
// append-only list of user/assistant text turns -- never an image block,
// so it can never carry a screenshot however long the run runs, and never
// edited in place, so it stays compatible with providers that reject an
// edited history.
export class AnthropicPersonaConversation implements PersonaConversation {
	private readonly client: MessagesClient;
	private readonly model: string;
	private readonly system: string;
	private readonly history: Anthropic.MessageParam[] = [];
	readonly slug: string;

	constructor(slug: string, options: AnthropicPersonaConversationOptions) {
		this.slug = slug;
		this.client = options.client;
		this.model = options.model ?? DEFAULT_MODEL;
		// The judgment rubric and the two reply formats are fixed for the
		// whole run, so they belong in the frozen system prompt rather than
		// being re-sent on every act (as judgmentPrompt/narrateWaitPrompt did
		// originally) -- shorter per-turn history, and a stable prefix a
		// caller can point cache_control at.
		this.system = `${options.personaBrief}\n\n${OUTCOME_RUBRIC}`;
	}

	private async send(userText: string): Promise<string> {
		this.history.push({ role: 'user', content: userText });
		const response = await this.client.messages.create({
			model: this.model,
			max_tokens: 1024,
			system: this.system,
			messages: this.history
		});
		const text = response.content.find((block): block is Anthropic.TextBlock => block.type === 'text')?.text ?? '';
		this.history.push({ role: 'assistant', content: text });
		return text;
	}

	async judge(record: PersonaTurnRecord): Promise<PersonaJudgment> {
		const response = await this.send(judgmentPrompt(record));
		return parseJudgment(response);
	}

	async narrateWait(timingMs: number): Promise<string> {
		const response = await this.send(narrateWaitPrompt(timingMs));
		return response.trim();
	}

	record(result: PersonaTurnResult): void {
		// A plain noted exchange, not a question -- record() never asks her
		// anything, it only keeps her history in step with what the log
		// ended up saying (which can differ from her own judge() answer,
		// when the performance band bumped the outcome afterwards).
		this.history.push({ role: 'user', content: `Recorded for your own memory: ${describeResult(result)}` }, { role: 'assistant', content: 'Noted.' });
	}
}

function describeAct(record: PersonaTurnRecord): string {
	return JSON.stringify({ id: record.id, act: record.act, result: record.result });
}

function describeResult(result: PersonaTurnResult): string {
	return JSON.stringify({ id: result.id, act: result.act, result: result.result, outcome: result.outcome, narrated: result.narrated ?? 'none' });
}

// Fixed for the whole run -- part of the system prompt (see the
// constructor), not resent per act.
const OUTCOME_RUBRIC = [
	"You are asked, after each act you perform, to judge its Outcome from your own read of what happened. Use exactly one of these four values:",
	'- "completed": you did the thing you were trying to do, and nothing cost you.',
	'- "completed with friction": you did it, but it cost you -- a retry, a guess, a back-navigation, a wait, a second screen you had to visit to find out whether the first one worked.',
	'- "refused": the product deliberately said no.',
	'- "stuck": you could not do it at all.',
	'',
	'When asked to judge an act, reply with JSON only, no prose, no markdown fence: {"outcome": "<one of the four values>", "narrated": "<string or null>"}. "narrated" is a short, first-person, present-tense line in your own voice: what you were trying to do, what you thought was happening, what you did next. It is required for every outcome except "completed", where it must be null.',
	'',
	"When asked instead to name a wait on an act that already completed but crossed the harness's own 1 second friction band -- a judgment that is not yours to make or argue with -- reply with plain text only, no JSON, no quotation marks: one short, first-person, present-tense sentence naming the wait."
].join('\n');

function judgmentPrompt(record: PersonaTurnRecord): string {
	return `You just attempted this act: ${describeAct(record)}\n\nJudge its Outcome now, per the format you were given.`;
}

function narrateWaitPrompt(timingMs: number): string {
	return `Your last act completed, but it took ${timingMs} ms, crossing the 1 second friction band. Name the wait now, per the format you were given.`;
}

function extractJSON(text: string): string {
	const fenced = /```(?:json)?\s*([\s\S]*?)```/i.exec(text);
	return (fenced ? fenced[1] : text).trim();
}

function parseJudgment(response: string): PersonaJudgment {
	let parsed: unknown;
	try {
		parsed = JSON.parse(extractJSON(response));
	} catch {
		throw new Error(`persona-conversation: could not parse a judgment out of: ${response}`);
	}
	if (typeof parsed !== 'object' || parsed === null || !('outcome' in parsed)) {
		throw new Error(`persona-conversation: judgment carried no outcome: ${response}`);
	}
	const { outcome } = parsed as { outcome: unknown };
	if (typeof outcome !== 'string' || !OUTCOMES.has(outcome as Outcome)) {
		throw new Error(`persona-conversation: judgment named an outcome this format doesn't have: ${response}`);
	}
	const narratedRaw = (parsed as { narrated?: unknown }).narrated;
	const narrated = typeof narratedRaw === 'string' && narratedRaw.length > 0 ? narratedRaw : undefined;
	return { outcome: outcome as Outcome, narrated };
}

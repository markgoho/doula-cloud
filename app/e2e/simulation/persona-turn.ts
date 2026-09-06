// Wraps one Persona's act for observedAct (#821): the script performs
// the Playwright interaction and reports what happened; her own
// conversation -- never the script -- judges the Outcome and, when
// there's friction, narrates it. capture.ts's own narrateWait hook is
// wired to the same conversation, since a band-induced friction bump is
// the one case it can still recover after the fact.
import type { Page } from '@playwright/test';
import type { ActOutcome, ActSpec, Capture, ObserveOptions } from './capture';
import { observedAct } from './capture';
import type { PersonaConversation } from './persona-conversation';

// What the script performing a Persona's act reports: what actually
// happened, the raw material for her voice -- never a verdict on the
// Outcome, which is the conversation's job, not the script's.
export interface PersonaActScript {
	result: string;
	evidence?: ActOutcome['evidence'];
}

export async function personaAct(
	page: Page,
	spec: ActSpec,
	conversation: PersonaConversation,
	script: () => Promise<PersonaActScript>,
	// 'narration' is excluded, not just left at its default: a Persona
	// always owes the Narrated register README.md defines, and must never
	// be handed the Extra's 'forbidden' setting.
	options: Omit<ObserveOptions, 'narrateWait' | 'slug' | 'narration'>
): Promise<Capture> {
	const perform = async (): Promise<ActOutcome> => {
		let scripted: PersonaActScript;
		try {
			scripted = await script();
		} catch (error) {
			// The product stopped her before the script could even report a
			// result. She is stuck, and her own conversation still gets to
			// say so in her voice -- exactly as if the script had reported
			// the failure as an ordinary Result instead of throwing.
			const message = error instanceof Error ? error.message : String(error);
			const judgment = await conversation.judge({ id: spec.id, act: spec.act, result: message });
			return { result: message, outcome: judgment.outcome, narrated: judgment.narrated };
		}
		const judgment = await conversation.judge({ id: spec.id, act: spec.act, result: scripted.result });
		return { result: scripted.result, outcome: judgment.outcome, narrated: judgment.narrated, evidence: scripted.evidence };
	};

	const capture = await observedAct(page, spec, perform, {
		...options,
		slug: conversation.slug,
		narrateWait: (timingMs) => conversation.narrateWait(timingMs)
	});

	if (capture.ok) {
		// Keeps her held history in step with what the log actually ended
		// up saying -- which can differ from her own judge() answer, when
		// the performance band bumped a 'completed' judgment into friction
		// afterward.
		conversation.record({
			id: capture.entry.id,
			act: capture.entry.act,
			result: capture.entry.result,
			outcome: capture.entry.outcome,
			narrated: capture.entry.narrated
		});
	}

	return capture;
}

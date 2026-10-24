import { error } from '@sveltejs/kit';
import { TERMS } from '#lib/legal.js';
import { versionWithText, versionsWithText } from '#lib/legalTexts.js';
import type { EntryGenerator, PageLoad } from './$types';

// One page for each version of the Terms of Service that has a text, at
// the date it takes effect (#1816): the email before a material change
// links the coming one, and a Practice's agreement names one by its date.
export const entries: EntryGenerator = () => versionsWithText(TERMS).map(({ effective }) => ({ version: effective }));

export const load: PageLoad = ({ params }) => {
	const version = versionWithText(TERMS, params.version);
	// The prerenderer only asks for the dates entries() named, so this is
	// reached only by a dev server asked for a version with no text.
	if (!version) error(404);
	return { version };
};

/**
The key the portal Engagement layout's `load` registers with `depends`, so
the hub can read the Engagement again with `invalidate` (#2587).
*/
export const PORTAL_ENGAGEMENT_DEPENDENCY = 'portal:engagement';

const reads = { made: 0, seenByHub: -1 };

/**
The layout's `load` read the Engagement.
*/
export function noteEngagementRead(): void {
	reads.made += 1;
}

/**
Whether the Engagement the layout holds was read since the hub last drew.
The layout keeps its data across the Engagement's pages, so a hub reached
from the Contract or the Messages holds an older read than one reached by
signing in, and reads again.
*/
export function isEngagementReadNew(): boolean {
	const isFresh = reads.made !== reads.seenByHub;
	reads.seenByHub = reads.made;
	return isFresh;
}

import type { OriginList } from './listOrigin.svelte.js';

/**
 * The list the open Engagement record was reached from (#2755), for the top
 * bar to mark current. Written by `ListBack` as each navigation lands; none
 * means the Clients list, where a cold load, the Overview and the Clients
 * list itself all lead.
 */
export const recordOrigin = $state<{ list: OriginList | undefined }>({ list: undefined });

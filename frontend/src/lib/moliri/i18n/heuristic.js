// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Mirrored from Moliri (src/lib/i18n/heuristic.js) by frontend/scripts/sync-moliri.mjs.
// Do not change it here: change it in Moliri, then pnpm sync:moliri.

import { svelteDefaultHeuristic } from '@wuchale/svelte';

/**
 * Which strings wuchale takes as interface text on its own (migration plan, hard spot 2).
 *
 * Automatic only for text nodes and for static values of attributes whose name says they hold
 * text. Everything else, including every string inside `{…}` in markup and all script scope,
 * is taken only with a `@wc-include` mark: there sit API endpoints, header and key names and
 * comparison values that must not be translated. Shared by `wuchale.config.js` and the codemod,
 * so the two cannot drift apart.
 */

// Attributes that hold text: HTML ones plus the prop names the code uses for texts
// (title, aria-label, placeholder, titel, text, label, description …).
const TEXT_ATTR =
	/^(alt|aria-label|aria-description|aria-placeholder|aria-roledescription|aria-valuetext|placeholder|title|titel|label|text|hint|description)$|(Label|Title|Titel|Text|Hint|Message|Description|Placeholder|Tooltip)$/;
const NO_TEXT_ELEMENTS = new Set(['kbd', 'code', 'pre', 'samp', 'var']);

/** @type {import('wuchale').HeuristicFunc} */
export const moliriHeuristic = (txt, file) => {
	if (txt.path.some((s) => s.type === 'element' && NO_TEXT_ELEMENTS.has(s.name))) return false;
	if (svelteDefaultHeuristic(txt, file) !== 'message') return false; // no URLs: we do not localize them
	const last = txt.path.at(-1);
	const body = Array.isArray(txt.body) ? txt.body.join('\n') : txt.body;
	if (last?.type === 'element')
		return (body.match(/\p{L}/gu) ?? []).length >= 2 ? 'message' : false;
	if (last?.type === 'attribute') return TEXT_ATTR.test(last.name) ? 'message' : false;
	return false; // script scope and everything inside {…}: only with @wc-include
};

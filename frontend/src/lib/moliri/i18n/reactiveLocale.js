// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Mirrored from Moliri (src/lib/i18n/reactiveLocale.js) by frontend/scripts/sync-moliri.mjs.
// Do not change it here: change it in Moliri, then pnpm sync:moliri.

import { createSubscriber } from 'svelte/reactivity';
import { getLocale, onLocaleChange } from './locale.js';

/**
 * `getLocale()` that Svelte tracks: read in a template, `$derived` or `$effect`, it is read again
 * after each `setLocale` (the language switch, no reload). For texts outside wuchale that are
 * picked by language in code, such as the field type descriptions (`fieldTypes/FieldTypeDoc.svelte`).
 *
 * Kept out of `locale.js`, which Node scripts and the media worker import without Svelte.
 */
const subscribe = createSubscriber((update) => onLocaleChange(update));

export function reactiveLocale() {
	subscribe();
	return getLocale();
}

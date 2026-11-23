// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { loadLocale } from 'wuchale/load-utils';
import { getLocale } from '$lib/moliri/i18n/locale.js';
// Registers wuchale's catalogue loaders before the first load, so every component and plain module
// it transformed finds its texts already there (as in Moliri). A loader registered later starts
// empty and stays so until the next loadLocale.
import '../locales/main.loader.svelte.js';
import '../locales/js.loader.js';

// A pure SPA in the browser: the EXE serves the same index.html for every page.
export const ssr = false;
export const prerender = false;

/** Loads the wuchale catalogue of the interface language before the app renders. */
export const load = async () => {
	document.documentElement.lang = getLocale();
	await loadLocale(getLocale());
};

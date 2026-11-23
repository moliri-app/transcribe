// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// wuchale, as in Moliri: interface texts as English source text in the code, translations in
// src/locales/<locale>.po. Components and .svelte.js files go through the Svelte adapter, plain
// modules through the vanilla one. Which strings count as text is Moliri's heuristic (mirrored):
// text nodes and text attributes on their own; in script scope and inside {…} in markup only
// literals marked with /* @wc-include */.
//
// The mirrored Moliri components carry English texts like everything else; scripts/sync-moliri.mjs
// copies Moliri's translations of their texts into the catalogues, so they read as in Moliri.
import { defineConfig } from 'wuchale';
import { adapter as svelte, defaultArgs } from '@wuchale/svelte';
import { adapter as js } from 'wuchale/adapter-vanilla';
import { moliriHeuristic } from './src/lib/moliri/i18n/heuristic.js';
import { locales } from './src/lib/moliri/i18n/locale.js';

// In class methods (the editor's state classes in .svelte.js) wuchale would declare a $derived
// inside each method and read it at once (`state_referenced_locally`). The plain runtime reads the
// same catalogue at call time, and as the loader keeps it in $state, a getter read in a template
// still follows a language switch. (Same as Moliri.)
const inMethod = (/** @type {{ type: string }[]} */ path) => path.some((s) => s.type === 'method');
const svelteRuntime = {
	...defaultArgs.runtime,
	initReactive: (/** @type {any} */ path, /** @type {string} */ file, /** @type {any} */ ctx) =>
		inMethod(path) ? false : defaultArgs.runtime.initReactive(path, file, ctx),
	useReactive: (/** @type {any} */ path, /** @type {string} */ file, /** @type {any} */ ctx) =>
		inMethod(path) ? false : defaultArgs.runtime.useReactive(path, file, ctx)
};

export default defineConfig({
	locales: [...locales], // first = source locale (en)
	ai: null, // translations are written by hand
	adapters: {
		main: svelte({
			// SSR is off (src/routes/+layout.js): the client-only loader.
			loader: 'svelte',
			heuristic: moliriHeuristic,
			runtime: svelteRuntime,
			files: { include: ['src/**/*.svelte', 'src/**/*.svelte.{js,ts}'], ignore: ['src/locales/**'] }
		}),
		js: js({
			// Our own loader keeps the runtimes in $state, so texts from plain modules follow a
			// language switch (src/locales/js.loader.js); wuchale does not rewrite it.
			loader: 'custom',
			heuristic: () => false, // plain modules: only texts marked with @wc-include
			files: {
				include: ['src/**/*.{js,ts}'],
				ignore: ['**/*.d.ts', '**/*.svelte.{js,ts}', 'src/locales/**', 'src/lib/generated/**', 'src/lib/moliri/i18n/**']
			}
		})
	}
});

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import Icons from 'unplugin-icons/vite';
import IconsResolver from 'unplugin-icons/resolver';
import AutoImport from 'unplugin-auto-import/vite';
import { wuchale } from 'wuchale/vite';
import { wuchaleBuild } from './scripts/wuchale-build.mjs';

// Wohin `pnpm dev` die API weiterreicht: die laufende EXE (Standard-Port 31765).
const backend = process.env.TRANSCRIBE_URL || 'http://localhost:31765';

// wuchale (wuchale.config.js) translates the interface texts, as in Moliri. The app is a pure SPA,
// so the plugin always transforms for the client. A build gets compiled catalogues of its own
// (scripts/wuchale-build.mjs): next to a running dev server, wuchale's own plugin would bundle the
// dev server's and shift every text.
function wuchaleClientOnly(command) {
	if (command === 'build') return wuchaleBuild({ root: import.meta.dirname });
	const plugin = wuchale();
	const handler = plugin.transform.handler;
	plugin.transform.handler = function (code, id, options) {
		return handler.call(this, code, id, { ...options, ssr: false });
	};
	return plugin;
}

export default defineConfig(({ command }) => ({
	plugins: [
		wuchaleClientOnly(command),
		sveltekit(),
		// Icons wie in Moliri: <IconMaterialSymbolsClose /> ohne Import, aus @iconify-json/<sammlung>.
		AutoImport({
			resolvers: [IconsResolver({ prefix: 'Icon' })],
			dts: './auto-imports.d.ts'
		}),
		Icons({
			compiler: 'svelte',
			defaultStyle: 'margin: 0 0.25em; padding: 0.07em;'
		})
	],
	server: {
		proxy: Object.fromEntries(['/api', '/audio', '/results', '/web'].map((p) => [p, { target: backend, changeOrigin: true }]))
	},
	css: {
		preprocessorOptions: {
			scss: { loadPaths: ['src'] }
		}
	}
}));

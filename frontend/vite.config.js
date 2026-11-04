// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import Icons from 'unplugin-icons/vite';
import IconsResolver from 'unplugin-icons/resolver';
import AutoImport from 'unplugin-auto-import/vite';

// Wohin `pnpm dev` die API weiterreicht: die laufende EXE (Standard-Port 8765).
const backend = process.env.TRANSCRIBE_URL || 'http://localhost:8765';

export default defineConfig({
	plugins: [
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
});

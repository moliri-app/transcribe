// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		// Die EXE bettet web/dist ein (web/web.go). Eine reine SPA: jede neue Seite liefert der
		// Server als index.html aus, welche das ist, steht in internal/server/pages.go.
		adapter: adapter({
			pages: '../web/dist',
			assets: '../web/dist',
			fallback: 'index.html'
		})
	}
};

export default config;

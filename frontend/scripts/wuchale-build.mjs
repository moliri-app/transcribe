// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Copied from Moliri (scripts/wuchale-build.mjs); keep it equal to Moliri's.

// wuchale for `vite build`, with compiled catalogues of the build's own.
//
// wuchale compiles each catalogue into an array (src/locales/.wuchale/main.0.de.compiled.js) and
// turns every text in the code into an index into it. The dev server numbers the texts its own way:
// it keeps obsolete entries and appends the texts it meets during the session, while a build leaves
// the obsolete ones out. With wuchale's own plugin, a build next to the workspace's running Vite
// finds that Vite's pid in src/locales/.wuchale/dev.pid, runs as a "secondary process" with a
// read-only file system and bundles the catalogues the dev server compiled, but indexes them with
// its own numbers: every text is shifted ("Brown" showed "Das Bild ließ sich nicht laden.").
// Writing the build's catalogues into src/locales/.wuchale/ instead would shift the dev server's.
//
// So a build writes its compiled catalogues and proxies into a directory of its own
// (node_modules/.wuchale-build/) and resolves the loaders' imports of `./.wuchale/…` there. It
// never writes into src/: the PO files, the loaders and data.js stay as committed (a running dev
// server is the only one to update them). This mirrors wuchale/vite's plugin (0.26.7) otherwise.
import fs from 'node:fs';
import path from 'node:path';
import { Hub, generatedDir, getConfig } from 'wuchale';
import { toViteError, trimViteQueries } from 'wuchale/vite';

export const BUILD_CATALOGUE_DIR = 'node_modules/.wuchale-build';

const trimParams = new Set(['v', 't', 'sentry-auto-wrap', 'tsr-split']);

/**
 * The file system wuchale's hub gets in a build: paths in the generated directory go to `ownDir`,
 * writes anywhere else are dropped, and the dev server's pid file reads as absent, so the build
 * compiles its catalogues itself (as the "primary" process).
 * @param {string} genDir absolute src/locales/.wuchale
 * @param {string} ownDir absolute directory of the build's generated files
 */
export function buildFS(genDir, ownDir) {
	const own = (/** @type {string} */ file) => {
		const abs = path.resolve(file);
		if (abs === genDir) return ownDir;
		return abs.startsWith(genDir + path.sep) ? ownDir + abs.slice(genDir.length) : null;
	};
	return {
		/** @param {string} file */
		async read(file) {
			const target = own(file);
			if (target && path.basename(target) === 'dev.pid') return null;
			try {
				return fs.readFileSync(target ?? file, 'utf-8');
			} catch (err) {
				if (/** @type {NodeJS.ErrnoException} */ (err).code === 'ENOENT') return null;
				throw err;
			}
		},
		/** @param {string} file @param {string} content */
		async write(file, content) {
			const target = own(file);
			if (!target) return;
			// SvelteKit's passes each write the same files: skip unchanged ones, write the others in
			// one synchronous call (no temp file and rename to collide on Windows).
			try {
				if (fs.readFileSync(target, 'utf-8') === content) return;
			} catch {
				// not there yet
			}
			fs.mkdirSync(path.dirname(target), { recursive: true });
			fs.writeFileSync(target, content);
		},
		/** @param {string} dir */
		async mkdir(dir) {
			const target = own(dir);
			if (target) fs.mkdirSync(target, { recursive: true });
		},
		/** @param {string} file */
		async exists(file) {
			return fs.existsSync(own(file) ?? file);
		},
		/** @param {string} file */
		async unlink(file) {
			const target = own(file);
			if (target) fs.rmSync(target, { force: true });
			return true;
		}
	};
}

/**
 * The wuchale plugins for `vite build`.
 * @param {{ root?: string }} [opts] root: the project directory (wuchale.config.js, cwd by default)
 * @returns {import('vite').Plugin[]}
 */
export function wuchaleBuild({ root = process.cwd() } = {}) {
	/** @type {any} */
	let conf;
	/** @type {any} */
	let hub;
	let genDir = '';
	const ownDir = path.resolve(root, BUILD_CATALOGUE_DIR);
	return [
		{
			// Before Vite's own resolver: the loaders import `./.wuchale/<key>.proxy.js`.
			name: 'moliri-wuchale-build-catalogues',
			enforce: 'pre',
			resolveId(source, importer) {
				if (!importer || !genDir || !source.startsWith('.') || !source.includes(generatedDir)) return null;
				const abs = path.resolve(path.dirname(importer.split('?')[0]), source);
				if (!abs.startsWith(genDir + path.sep)) return null;
				return ownDir + abs.slice(genDir.length);
			}
		},
		{
			name: 'wuchale',
			async config() {
				conf = await getConfig(path.join(root, 'wuchale.config.js'));
				genDir = path.resolve(root, conf.localesDir, generatedDir);
			},
			async buildStart() {
				hub = await Hub.create('build', conf, root, [], 1000, buildFS(genDir, ownDir), toViteError);
			},
			transform: {
				order: 'pre',
				async handler(code, id) {
					// The app is a pure SPA: always the client loader (see wuchaleClientOnly in vite.config.js).
					return await hub.transform(code, trimViteQueries(id, trimParams), false);
				}
			}
		}
	];
}

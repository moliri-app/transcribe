// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

/**
 * Lizenzübersicht für die Seite /lizenzen erzeugen, nach Moliris scripts/generate-licenses.mjs.
 * Läuft vor dev, build und check und schreibt src/lib/generated/licenses.json (nicht eingecheckt).
 *
 *   frontend  was die Oberfläche mitbringt: pnpm licenses list --prod
 *   backend   die Go-Module, die in der EXE stecken: go list -deps ./cmd/transcribe, Lizenz aus
 *             der LICENSE-Datei des Moduls
 *   icons     die Icon-Sammlungen unter @iconify-json/* (unplugin-icons)
 *   fonts     die mitgelieferten Schriften
 *   notice    der Text von NOTICE (Laufzeit-Downloads, Modelle, Markenhinweis)
 */

import { exec as execCallback, execFile as execFileCallback } from 'node:child_process';
import { mkdir, readFile, readdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';

const execFile = promisify(execFileCallback);
const exec = promisify(execCallback);

const frontend = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const repo = path.resolve(frontend, '..');
const outputPath = path.join(frontend, 'src', 'lib', 'generated', 'licenses.json');
const win = process.platform === 'win32';

function normalizeLicenseName(license) {
	if (Array.isArray(license)) license = license.join(' OR ');
	if (typeof license !== 'string' || !license.trim()) return 'Unknown';
	license = license.trim();
	// bei Wahl zwischen MIT und anderem: MIT (wie Moliri)
	if (/\bMIT\b/i.test(license) && /\bOR\b/i.test(license)) return 'MIT';
	return license.replace(/^\((.*)\)$/, '$1');
}

/** { Lizenz: [{ name, version?, url?, note? }] }, nach Lizenz und Name sortiert */
function grouped(entries) {
	const out = {};
	for (const { license, ...entry } of entries) (out[license] ??= new Map()).set(entry.name, entry);
	return Object.fromEntries(
		Object.entries(out)
			.sort(([a], [b]) => a.localeCompare(b))
			.map(([license, m]) => [license, [...m.values()].sort((a, b) => a.name.localeCompare(b.name))])
	);
}

async function run(cmd, args, cwd) {
	const opts = { cwd, maxBuffer: 1024 * 1024 * 20 };
	// pnpm ist unter Windows eine .cmd-Datei und braucht die Shell (die Argumente hier sind fest)
	const { stdout } = win && cmd === 'pnpm' ? await exec([cmd, ...args].join(' '), opts) : await execFile(cmd, args, opts);
	return stdout;
}

async function frontendLicenses() {
	const own = JSON.parse(await readFile(path.join(frontend, 'package.json'), 'utf8')).name;
	const reported = JSON.parse(await run('pnpm', ['licenses', 'list', '--json', '--prod'], frontend));
	const entries = [];
	for (const [license, packages] of Object.entries(reported)) {
		for (const p of packages ?? []) {
			if (!p?.name || p.name === own) continue;
			entries.push({
				license: normalizeLicenseName(p.license ?? license),
				name: p.name,
				version: p.versions?.join(', '),
				url: p.homepage ?? null
			});
		}
	}
	return grouped(entries);
}

/** Lizenz aus dem Text einer LICENSE-Datei, für die wenigen Go-Module */
function classify(text) {
	if (/Apache License[\s\S]*Version 2\.0/i.test(text)) return 'Apache-2.0';
	if (/Mozilla Public License,? (?:v\.|version) ?2\.0/i.test(text)) return 'MPL-2.0';
	if (/Permission is hereby granted, free of charge/i.test(text)) return 'MIT';
	if (/Redistribution and use in source and binary forms/i.test(text)) {
		return /Neither the name/i.test(text) ? 'BSD-3-Clause' : 'BSD-2-Clause';
	}
	// both wordings of the ISC licence: "and/or distribute" and the older "and distribute"
	if (/Permission to use, copy, modify, and(?:\/or)? distribute/i.test(text)) return 'ISC';
	return 'Unknown';
}

async function backendLicenses() {
	let out;
	try {
		out = await run('go', ['list', '-deps', '-f', '{{with .Module}}{{.Path}}\t{{.Version}}\t{{.Dir}}{{end}}', './cmd/transcribe'], repo);
	} catch (e) {
		console.warn(`Go-Module übersprungen (go list ging nicht: ${e.message.split('\n')[0]})`);
		return {};
	}
	const own = (await readFile(path.join(repo, 'go.mod'), 'utf8')).match(/^module\s+(\S+)/m)?.[1];
	const modules = new Map();
	for (const line of out.split(/\r?\n/)) {
		const [name, version, dir] = line.split('\t');
		if (name && name !== own) modules.set(name, { version, dir });
	}
	const entries = [];
	for (const [name, { version, dir }] of modules) {
		const files = await readdir(dir).catch(() => []);
		const file = files.find((f) => /^(LICENSE|LICENCE|COPYING)(\.|$)/i.test(f));
		const license = file ? classify(await readFile(path.join(dir, file), 'utf8')) : 'Unknown';
		entries.push({ license, name, version, url: `https://pkg.go.dev/${name}` });
	}
	return grouped(entries);
}

async function iconLicenses() {
	const pkg = JSON.parse(await readFile(path.join(frontend, 'package.json'), 'utf8'));
	const collections = Object.keys({ ...pkg.dependencies, ...pkg.devDependencies }).filter((n) => n.startsWith('@iconify-json/'));
	const entries = [];
	for (const c of collections) {
		const info = JSON.parse(await readFile(path.join(frontend, 'node_modules', c, 'info.json'), 'utf8'));
		entries.push({
			license: normalizeLicenseName(info.license?.spdx ?? info.license?.title),
			name: `${info.name} (${info.prefix})`,
			url: info.license?.url ?? info.author?.url ?? null,
			note: `Über unplugin-icons (<Icon${info.prefix.replace(/(^|-)(\w)/g, (_, __, ch) => ch.toUpperCase())}… />).${info.author?.name ? ` Von ${info.author.name}.` : ''}`
		});
	}
	return grouped(entries);
}

async function fontLicenses() {
	const entries = [];
	// Atkinson Hyperlegible Next und Mono über Fontsource (wie in Moliri); der Text der OFL liegt in static/OFL.txt
	const pkg = (/** @type {string} */ name) =>
		readFile(path.join(frontend, 'node_modules', '@fontsource-variable', name, 'package.json'), 'utf8').then(JSON.parse);
	const next = await pkg('atkinson-hyperlegible-next');
	const mono = await pkg('atkinson-hyperlegible-mono');
	entries.push({
		license: normalizeLicenseName(next.license),
		name: 'Atkinson Hyperlegible Next',
		version: next.version,
		url: 'https://github.com/googlefonts/atkinson-hyperlegible-next',
		note: 'Schrift der Oberfläche, mitgeliefert über @fontsource-variable/atkinson-hyperlegible-next (variabel). Lizenztext unter /OFL.txt.'
	});
	entries.push({
		license: normalizeLicenseName(mono.license),
		name: 'Atkinson Hyperlegible Mono',
		version: mono.version,
		url: 'https://github.com/googlefonts/atkinson-hyperlegible-next-mono',
		note: 'Schrift für Code, mitgeliefert über @fontsource-variable/atkinson-hyperlegible-mono (variabel). Lizenztext unter /OFL.txt.'
	});
	return grouped(entries);
}

async function main() {
	const data = {
		generatedAt: new Date().toISOString(),
		frontend: { source: 'pnpm licenses list --json --prod', licenses: await frontendLicenses() },
		backend: { source: 'go list -deps ./cmd/transcribe', licenses: await backendLicenses() },
		icons: { source: '@iconify-json/* in frontend/package.json', licenses: await iconLicenses() },
		fonts: { source: '@fontsource-variable/atkinson-hyperlegible-*', licenses: await fontLicenses() },
		notice: await readFile(path.join(repo, 'NOTICE'), 'utf8').then((t) => t.replace(/\r\n/g, '\n'))
	};
	await mkdir(path.dirname(outputPath), { recursive: true });
	await writeFile(outputPath, `${JSON.stringify(data, null, 2)}\n`, 'utf8');
	console.log(`Lizenzübersicht geschrieben: ${path.relative(frontend, outputPath)}`);
}

main().catch((error) => {
	console.error('Lizenzübersicht ging nicht.');
	console.error(error);
	process.exit(1);
});

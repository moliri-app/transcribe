<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Die Details einer Aufnahme in der Bibliothek: oben zum Anhören, darunter, was die Datei ist
     (Name, wo sie liegt, wie groß, wann erstellt und geändert) und was in ihr steckt (Länge, wann
     aufgenommen, Format, Ton und Bild; GET /api/fileinfo, das Letzte liest ffprobe), bei einer
     transkribierten auch der Stand ihres Transkripts. open(a) öffnet. -->
<script>
	import Button from '$lib/moliri/components/ui/Button.svelte';
	import Abspielen from '$lib/live/Abspielen.svelte';
	import { dauer, getJSON, groesse } from '$lib/api.js';
	import { getLocale } from '$lib/moliri/i18n/locale.js';
	import plural from '../locales/plural.js';

	/** @typedef {{ format: string, duration: number, bitrate: number, recorded: string, title: string, artist: string,
	 *   codec: string, sampleRate: number, channels: number, video?: { codec: string, width: number, height: number, fps: number } }} Medium */
	/** @typedef {{ file: string, path: string, size: number, modified: number, created?: number, media?: Medium }} Info */
	/** @typedef {{ name: string, stem: string, title: string, video: boolean, sidecar?: boolean,
	 *   summary?: { duration: number, speakers: unknown[], edits: number, clips: number, done: number } }} Zeigbar */

	/** oneditor: die Aufnahme im Editor öffnen (die Bibliothek merkt sie sich dafür)
	 * @type {{ oneditor?: (a: Zeigbar) => void }} */
	let { oneditor } = $props();

	/** @type {HTMLDialogElement} */
	let dlg;
	/** @type {Zeigbar | null} */
	let a = $state(null);
	/** @type {Info | null} */
	let info = $state(null);
	let fehler = $state('');

	/** @param {Zeigbar} aufnahme */
	export async function open(aufnahme) {
		a = aufnahme;
		info = null;
		fehler = '';
		dlg.showModal();
		try {
			const i = await getJSON(`/api/fileinfo?name=${encodeURIComponent(aufnahme.name)}`);
			if (a === aufnahme) info = i;
		} catch (e) {
			if (a === aufnahme) fehler = /** @type {Error} */ (e).message;
		}
	}

	/** 8. November 2026, 14:05
	 * @param {number | string | Date} t Sekunden, oder ein Datum, wie die Datei es nennt */
	function datum(t) {
		const d = typeof t === 'number' ? new Date(t * 1000) : new Date(t);
		if (isNaN(d.getTime())) return String(t);
		return d.toLocaleString(getLocale(), { day: 'numeric', month: 'long', year: 'numeric', hour: '2-digit', minute: '2-digit' });
	}

	/** @param {number} n */
	const kanaele = (n) => (n === 1 ? /* @wc-include */ 'Mono' : n === 2 ? /* @wc-include */ 'Stereo' : /* @wc-include */ `${n} channels`);

	/** der Ton in Worten: „AAC · 44,1 kHz · Stereo · 128 kbit/s“
	 * @param {Medium} m */
	function ton(m) {
		return [
			m.codec,
			m.sampleRate ? `${(m.sampleRate / 1000).toLocaleString(getLocale(), { maximumFractionDigits: 1 })} kHz` : '',
			m.channels ? kanaele(m.channels) : '',
			m.bitrate ? `${Math.round(m.bitrate / 1000)} kbit/s` : ''
		]
			.filter(Boolean)
			.join(' · ');
	}

	/** das Bild in Worten: „1920 × 1080 · H.264 · 30 Bilder/s“
	 * @param {NonNullable<Medium['video']>} v */
	function bild(v) {
		return [
			v.width && v.height ? `${v.width} × ${v.height}` : '',
			v.codec,
			v.fps ? fps(v.fps) : ''
		]
			.filter(Boolean)
			.join(' · ');
	}

	/** 30 fps
	 * @param {number} n */
	function fps(n) {
		const zahl = n.toLocaleString(getLocale(), { maximumFractionDigits: 2 });
		return /* @wc-include */ `${zahl} fps`;
	}

	/** @param {number} t */
	function fertigSeit(t) {
		const wann = datum(t);
		return /* @wc-include */ `ready since ${wann}`;
	}

	/** der Stand des Transkripts: „fertig seit 8. November 2026, 14:05 · 2 Sprecher · 3 Korrekturen“
	 * @param {NonNullable<Zeigbar['summary']>} s */
	function transkript(s) {
		return [
			fertigSeit(s.done),
			plural(s.speakers.length, ['# speaker', '# speakers']),
			s.edits ? plural(s.edits, ['# manual correction', '# manual corrections']) : '',
			s.clips ? plural(s.clips, ['# clip', '# clips']) : ''
		]
			.filter(Boolean)
			.join(' · ');
	}
</script>

<dialog class="details" bind:this={dlg} aria-labelledby="h-details" onclose={() => (a = null)}>
	{#if a}
		{@const m = info?.media}
		<h2 id="h-details">{a.title}</h2>
		<Abspielen datei={a.name} titel={a.title} />
		{#if fehler}
			<p class="fehler" role="alert">The file details could not be read: {fehler}</p>
		{:else if !info}
			<p class="leise">Reading the file…</p>
		{:else}
			<dl>
				<dt>File name</dt>
				<dd>{info.file}</dd>
				<dt>Location</dt>
				<dd class="pfad">{info.path}</dd>
				<dt>Size</dt>
				<dd>{groesse(info.size)}</dd>
				{#if m?.duration || a.summary}
					<dt>Length</dt>
					<dd class="zahl">{dauer(m?.duration || a.summary?.duration || 0)}</dd>
				{/if}
				{#if m?.recorded}
					<dt>Recorded</dt>
					<dd>{datum(m.recorded)} <span class="leise">(according to the file)</span></dd>
				{/if}
				{#if info.created}
					<dt>Created</dt>
					<!-- eine kopierte Datei ist hier jünger als ihr Inhalt -->
					<dd>{datum(info.created)}{#if info.created > info.modified + 60}&nbsp;<span class="leise">(copied here)</span>{/if}</dd>
				{/if}
				<dt>Modified</dt>
				<dd>{datum(info.modified)}</dd>
				{#if m?.title || m?.artist}
					<dt>In the file</dt>
					<dd>{[m.title, m.artist].filter(Boolean).join(' · ')}</dd>
				{/if}
				{#if m?.format}
					<dt>Format</dt>
					<dd>{m.format}</dd>
				{/if}
				{#if m && ton(m)}
					<dt>Sound</dt>
					<dd>{ton(m)}</dd>
				{/if}
				{#if m?.video}
					<dt>Picture</dt>
					<dd>{bild(m.video)}</dd>
				{/if}
				<dt>Transcript</dt>
				<dd>
					{#if a.summary}
						{transkript(a.summary)}{#if a.sidecar}<br /><span class="leise">saved as a file next to the recording</span>{/if}
					{:else}
						none yet
					{/if}
				</dd>
			</dl>
			{#if !m}
				<p class="leise">What is in the file (length, format) cannot be read right now.</p>
			{/if}
		{/if}
		<div class="fuss">
			{#if a.summary && oneditor}
				{@const z = a}
				<a class="editor" href="/editor" data-sveltekit-reload onclick={() => oneditor(z)}>
					<IconMaterialSymbolsEditOutlineRounded aria-hidden="true" />Open in editor
				</a>
			{/if}
			<span class="platz"></span>
			<Button type="secondary" size="large" autofocus onclick={() => dlg.close()}>Close</Button>
		</div>
	{/if}
</dialog>

<style>
	.details {
		width: min(38rem, calc(100vw - 2rem));
		margin: auto;
		padding: 1.5rem 1.75rem;
		border: 0;
		border-radius: 0.75rem;
		background: hsl(var(--color-bg-primary));
		color: hsl(var(--color-text-primary));
		box-shadow: 0 24px 64px -24px hsl(var(--color-neutral-100) / 0.5);
	}
	.details[open] {
		display: grid;
		gap: 1rem;
	}
	.details::backdrop {
		background: hsl(var(--color-neutral-100) / 0.3);
		backdrop-filter: blur(4px);
	}
	h2 {
		font-size: 1.375rem;
		letter-spacing: -0.01em;
		overflow-wrap: anywhere;
	}
	/* Angaben als Zweispalter: Name in ink-2, Wert in Tinte */
	dl {
		display: grid;
		grid-template-columns: max-content minmax(0, 1fr);
		gap: 0.5rem 1.25rem;
		margin: 0;
		padding-top: 1rem;
		border-top: 1px solid hsl(var(--color-border-primary));
	}
	dt {
		color: hsl(var(--color-text-secondary));
	}
	dd {
		margin: 0;
		overflow-wrap: anywhere;
	}
	.pfad {
		font-size: 0.9375rem;
		user-select: all;
	}
	.zahl {
		font-variant-numeric: tabular-nums;
	}
	.leise {
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
	}
	.fehler {
		color: hsl(var(--color-text-error));
	}
	.fuss {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem;
		margin-top: 0.5rem;
	}
	.platz {
		flex: 1;
	}
	.editor {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		min-height: var(--hit);
		padding: 0 0.75rem;
		margin-left: -0.75rem;
		border-radius: 2rem;
		font-weight: 600;
		color: hsl(var(--color-text-primary));
		text-decoration: none;
	}
	.editor:hover {
		background: hsl(var(--color-bg-secondary));
	}
	.editor :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
	}

	@media (max-width: 480px) {
		dl {
			grid-template-columns: minmax(0, 1fr);
			gap: 0.1rem;
		}
		dd {
			margin-bottom: 0.5rem;
		}
	}
</style>

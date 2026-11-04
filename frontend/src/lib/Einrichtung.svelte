<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Was auf jeder Seite mitläuft (Port von web/setup.js):
     - die Leiste der Ersteinrichtung, solange Laufzeit, FFmpeg und Modelle noch laden, und die
       Meldung, wenn ein Download scheitert;
     - einmalig die Frage, ob eine erkannte NVIDIA-GPU genutzt werden soll;
     - Dateien und Ordner, die ins Fenster gezogen werden: hochladen und einreihen, je nach
       Einstellung sofort oder nach einer Rückfrage mit den Optionen. -->
<script module>
	/** Startet das Beobachten der Einrichtung neu, etwa nachdem die GPU-Laufzeit angefordert wurde. */
	export const einrichtung = $state({ neu: 0 });
</script>

<script>
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import Modal from '$lib/moliri/components/ui/Modal.svelte';
	import Button from '$lib/moliri/components/ui/Button.svelte';
	import AuftragsOptionen from '$lib/AuftragsOptionen.svelte';
	import { geaendert } from '$lib/auftraege.svelte.js';
	import { MEDIA, einreihen, getJSON, post, upload, vorgaben } from '$lib/api.js';

	/** @typedef {{ label: string, state: string, done: number, total: number, error?: string }} Teil */

	// ---- Ersteinrichtung ----
	/** null: nichts zu zeigen */
	let stand = $state(/** @type {{ text: string, pct: number, fehler: boolean, fertig: boolean } | null} */ (null));
	let beobachtet = false;

	const mb = (/** @type {number} */ n) => (n / 1048576).toFixed(0) + ' MB';

	async function beobachten() {
		beobachtet = true;
		/** @type {{ ready: boolean, items?: Teil[] }} */
		let d;
		try {
			d = await getJSON('/api/setup');
		} catch {
			setTimeout(beobachten, 2000);
			return;
		}
		const items = d.items || [];
		const fertig = items.filter((i) => i.state === 'done').length;
		const fehler = items.find((i) => i.state === 'error');
		const laedt = items.find((i) => i.state === 'downloading');
		if (d.ready) {
			beobachtet = false;
			if (stand) {
				stand = { text: 'Alles bereit.', pct: 100, fehler: false, fertig: true };
				setTimeout(() => (stand = null), 1500);
			}
			return;
		}
		if (fehler) {
			beobachtet = false;
			stand = {
				text: `Download fehlgeschlagen (${fehler.label}): ${fehler.error}`,
				pct: 0,
				fehler: true,
				fertig: false
			};
			return;
		}
		let text = `Erste Einrichtung (${fertig}/${items.length})`;
		let pct = items.length ? (fertig / items.length) * 100 : 0;
		if (laedt) {
			text += `: lade ${laedt.label}`;
			if (laedt.total > 0) {
				text += ` – ${mb(laedt.done)} / ${mb(laedt.total)}`;
				pct += (laedt.done / laedt.total) * (100 / items.length);
			}
		} else {
			text += ' …';
		}
		stand = { text, pct: Math.min(100, pct), fehler: false, fertig: false };
		setTimeout(beobachten, 800);
	}

	$effect(() => {
		if (einrichtung.neu && !beobachtet) setTimeout(beobachten, 600);
	});

	// ---- GPU ----
	/** @type {{ gpu: { name: string, cudaMajor: number, ready: boolean, fetch?: number } } | null} */
	let gpu = $state(null);

	async function gpuAnbieten() {
		try {
			const g = await getJSON('/api/gpu');
			if (g.offer) gpu = g;
		} catch {
			// ohne Antwort fragen wir beim nächsten Start
		}
	}

	/** @param {boolean} ja */
	async function gpuWahl(ja) {
		gpu = null;
		try {
			await post(ja ? '/api/gpu/enable' : '/api/gpu/dismiss');
			if (ja) {
				sagen({ text: 'Die GPU-Laufzeit wird geladen. Den Fortschritt siehst du unten und unter', link: { href: '/settings', text: 'Einstellungen' } });
				einrichtung.neu++;
			}
		} catch (e) {
			sagen({ text: 'Fehler: ' + /** @type {Error} */ (e).message });
		}
	}

	// ---- Hinweis unten rechts ----
	/** @type {{ text: string, link?: { href: string, text: string } } | null} */
	let hinweis = $state(null);
	/** @type {ReturnType<typeof setTimeout> | undefined} */
	let hinweisUhr;
	/**
	 * @param {{ text: string, link?: { href: string, text: string } }} h
	 * @param {boolean} [bleibt] stehen lassen, bis der nächste Hinweis kommt
	 */
	function sagen(h, bleibt = false) {
		hinweis = h;
		clearTimeout(hinweisUhr);
		if (!bleibt) hinweisUhr = setTimeout(() => (hinweis = null), 6000);
	}

	// ---- Ablegen ----
	let ziehen = $state(false);
	let tiefe = 0;
	const mitDateien = (/** @type {DragEvent} */ e) => [...(e.dataTransfer?.types || [])].includes('Files');
	const aufDateifeld = (/** @type {DragEvent} */ e) => e.target instanceof Element && !!e.target.closest('input[type=file]');

	/** @param {DragEvent} e */
	function dragenter(e) {
		if (!mitDateien(e) || aufDateifeld(e)) return;
		e.preventDefault();
		tiefe++;
		ziehen = true;
	}
	/** @param {DragEvent} e */
	function dragover(e) {
		if (!mitDateien(e) || aufDateifeld(e)) return;
		e.preventDefault();
		if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy';
	}
	/** @param {DragEvent} e */
	function dragleave(e) {
		if (!mitDateien(e)) return;
		if (--tiefe <= 0) {
			tiefe = 0;
			ziehen = false;
		}
	}
	/** @param {DragEvent} e */
	function drop(e) {
		if (!mitDateien(e) || aufDateifeld(e)) return;
		e.preventDefault();
		tiefe = 0;
		ziehen = false;
		// die Einträge nur jetzt lesen: nach dem ersten await leert der Browser sie
		const wurzeln = [...(e.dataTransfer?.items ?? [])]
			.filter((i) => i.kind === 'file')
			.map((i) => ({ entry: i.webkitGetAsEntry?.() ?? null, file: i.getAsFile() }));
		abgelegt(wurzeln);
	}

	/**
	 * Dateien und Ordner (rekursiv) zu Mediendateien auflösen.
	 * @param {FileSystemEntry} entry
	 * @param {File[]} out
	 * @param {{ skipped: number }} stats
	 */
	async function durchgehen(entry, out, stats) {
		if (entry.isFile) {
			const f = await new Promise((ok, no) => /** @type {FileSystemFileEntry} */ (entry).file(ok, no)).catch(() => null);
			if (!f) return;
			if (MEDIA.test(f.name)) out.push(f);
			else stats.skipped++;
		} else if (entry.isDirectory) {
			const reader = /** @type {FileSystemDirectoryEntry} */ (entry).createReader();
			for (;;) {
				/** @type {FileSystemEntry[]} */
				const stapel = await new Promise((ok, no) => reader.readEntries(ok, no)).catch(() => []);
				if (!stapel.length) break;
				for (const c of stapel) await durchgehen(c, out, stats);
			}
		}
	}

	/** @param {{ entry: FileSystemEntry | null, file: File | null }[]} wurzeln */
	async function abgelegt(wurzeln) {
		/** @type {File[]} */
		const dateien = [];
		const stats = { skipped: 0 };
		sagen({ text: 'Lese Dateien …' }, true);
		for (const w of wurzeln) {
			if (w.entry) await durchgehen(w.entry, dateien, stats);
			else if (w.file) {
				if (MEDIA.test(w.file.name)) dateien.push(w.file);
				else stats.skipped++;
			}
		}
		if (!dateien.length) return sagen({ text: 'Keine Audio- oder Videodatei erkannt.' });
		/** @type {string[]} */
		const namen = [];
		try {
			for (const [i, f] of dateien.entries()) {
				sagen({ text: `Lade hoch (${i + 1}/${dateien.length}): ${f.name} …` }, true);
				namen.push(await upload(f));
			}
			hinweis = null;
			geaendert();
			const { dropMode, erkennung, ...o } = await vorgaben();
			/** @type {import('$lib/api.js').Optionen | null} */
			let optionen = o;
			if (dropMode !== 'auto') {
				optionen = await nachfragen(namen, o, stats.skipped, erkennung);
				if (!optionen) return sagen({ text: `${namen.length} Datei(en) hochgeladen. Transkribieren kannst du sie in der Bibliothek.` });
			}
			const n = await einreihen(namen, optionen);
			const uebersprungen = stats.skipped ? ` (${stats.skipped} übersprungen)` : '';
			sagen({
				text: `${n} Datei(en) werden transkribiert${uebersprungen}.`,
				link: ['/', '/queue'].includes(page.url.pathname) ? undefined : { href: '/', text: 'Zur Bibliothek' }
			});
			geaendert();
		} catch (err) {
			sagen({ text: 'Fehler: ' + /** @type {Error} */ (err).message });
		}
	}

	// Rückfrage nach dem Ablegen (außer „sofort starten“ ist eingestellt)
	/** @type {{ namen: string[], optionen: import('$lib/api.js').Optionen, skipped: number, sofort: boolean, erkennung: string,
	 *   fertig: (o: import('$lib/api.js').Optionen | null) => void } | null} */
	let frage = $state(null);

	/**
	 * @param {string[]} namen
	 * @param {import('$lib/api.js').Optionen} optionen
	 * @param {number} skipped
	 * @param {string} erkennung wo erkannt wird (Einstellungen)
	 * @returns {Promise<import('$lib/api.js').Optionen | null>}
	 */
	function nachfragen(namen, optionen, skipped, erkennung) {
		return new Promise((fertig) => (frage = { namen, optionen, skipped, erkennung, sofort: false, fertig }));
	}

	/** @param {boolean} ja */
	function beantworten(ja) {
		if (!frage) return;
		const f = frage;
		frage = null;
		if (ja && f.sofort) post('/api/prefs', { dropMode: 'auto' }).catch(() => {});
		f.fertig(ja ? f.optionen : null);
	}

	onMount(() => {
		beobachten();
		const t = setTimeout(gpuAnbieten, 1500);
		return () => clearTimeout(t);
	});
</script>

<svelte:window ondragenter={dragenter} ondragover={dragover} ondragleave={dragleave} ondrop={drop} />

{#if ziehen}
	<div class="ablage" aria-hidden="true">
		<div class="rahmen">
			<IconMaterialSymbolsUploadFileOutlineRounded class="symbol" />
			<strong>Dateien oder Ordner hier ablegen</strong>
			<span>Audio oder Video, kommt in die Bibliothek und wird transkribiert</span>
		</div>
	</div>
{/if}

{#if hinweis}
	<div class="hinweis" aria-live="polite">
		{hinweis.text}
		{#if hinweis.link}<a href={hinweis.link.href} data-sveltekit-reload>{hinweis.link.text}</a>{/if}
	</div>
{/if}

{#if stand}
	<div class="einrichtung" class:fehler={stand.fehler} role="region" aria-label="Ersteinrichtung" aria-live="polite">
		<div class="inhalt">
			{#if stand.fehler}
				<IconMaterialSymbolsErrorOutlineRounded class="symbol" aria-hidden="true" />
				<p>
					{stand.text} – Programm neu starten oder unter
					<a href="/settings" data-sveltekit-reload>Einstellungen</a> erneut laden.
				</p>
			{:else}
				<IconMaterialSymbolsDownloadRounded class="symbol" aria-hidden="true" />
				<p>
					{stand.text}{#if !stand.fertig}. Du kannst die Oberfläche schon nutzen; Aufträge starten, sobald alles bereit ist.{/if}
				</p>
			{/if}
		</div>
		{#if !stand.fehler}
			<div class="spur"><i style:width="{stand.pct}%"></i></div>
		{/if}
	</div>
{/if}

{#if gpu}
	<Modal title="NVIDIA-GPU erkannt" size="medium" onClose={() => (gpu = null)}>
		<div class="dialog">
			<p>
				<strong>{gpu.gpu.name}</strong> kann die Erkennung beschleunigen. Dafür lädt die App einmalig die GPU-Laufzeit
				(CUDA {gpu.gpu.cudaMajor}, etwa 500 MB){#if !gpu.gpu.ready && gpu.gpu.fetch}
					{' '}und die fehlenden Bibliotheken von NVIDIA (etwa {Math.round(gpu.gpu.fetch / 1048576)} MB){/if}.
			</p>
			<p>
				Wie viel das bringt, hängt vom Modell und der Aufnahme ab. Ohne GPU läuft alles wie bisher auf dem Prozessor,
				und du kannst das jederzeit in den Einstellungen ändern.
			</p>
			{#if !gpu.gpu.ready && gpu.gpu.fetch}
				<p class="lizenz">
					Mit „GPU-Laufzeit laden“ stimmst du den Lizenzbedingungen von NVIDIA zu, für das
					<a href="https://docs.nvidia.com/cuda/eula/" target="_blank" rel="noreferrer">CUDA Toolkit</a> und für
					<a href="https://docs.nvidia.com/deeplearning/cudnn/latest/reference/eula.html" target="_blank" rel="noreferrer">cuDNN</a>.
				</p>
			{/if}
		</div>
		{#snippet actions()}
			<Button type="secondary" size="large" onclick={() => gpuWahl(false)}>Nur Prozessor nutzen</Button>
			<Button type="primary" size="large" onclick={() => gpuWahl(true)}>GPU-Laufzeit laden</Button>
		{/snippet}
	</Modal>
{/if}

{#if frage}
	<Modal
		title={frage.namen.length === 1 ? '1 Datei verarbeiten' : `${frage.namen.length} Dateien verarbeiten`}
		size="medium"
		onClose={() => beantworten(false)}
	>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="dialog"
			onkeydown={(e) => {
				if (e.key === 'Enter' && e.target instanceof HTMLInputElement && e.target.type !== 'checkbox') beantworten(true);
			}}
		>
			<p>
				Die Dateien sind hochgeladen.{#if frage.skipped}
					{' '}{frage.skipped} andere Datei(en) wurden übersprungen.{/if}
			</p>
			<ul class="dateien">
				{#each frage.namen.slice(0, 8) as n (n)}<li>{n}</li>{/each}
				{#if frage.namen.length > 8}<li>… und {frage.namen.length - 8} weitere</li>{/if}
			</ul>
			<AuftragsOptionen bind:optionen={frage.optionen} erkennung={frage.erkennung} />
			<label class="sofort">
				<input type="checkbox" bind:checked={frage.sofort} />
				Künftig sofort starten, ohne Nachfrage
			</label>
		</div>
		{#snippet actions()}
			<Button type="secondary" size="large" onclick={() => beantworten(false)}>Später</Button>
			<Button type="primary" size="large" onclick={() => beantworten(true)}>Transkribieren</Button>
		{/snippet}
	</Modal>
{/if}

<style>
	.ablage {
		position: fixed;
		inset: 0;
		z-index: 10001;
		display: flex;
		align-items: center;
		justify-content: center;
		background: hsl(var(--color-neutral-100) / 0.72);
		color: hsl(var(--color-text-inverse));
		pointer-events: none;
	}
	.rahmen {
		display: grid;
		justify-items: center;
		gap: 0.5rem;
		padding: 3rem 4rem;
		border: 3px dashed hsl(var(--color-green-100));
		border-radius: 1.25rem;
		text-align: center;
	}
	.rahmen strong {
		font-size: 1.5rem;
	}
	.rahmen :global(.symbol) {
		width: 3rem;
		height: 3rem;
		color: hsl(var(--color-green-100));
	}

	.hinweis {
		position: fixed;
		right: 1rem;
		bottom: 5.5rem;
		z-index: 10000;
		max-width: min(26rem, calc(100vw - 2rem));
		padding: 0.85rem 1.1rem;
		border-radius: 0.75rem;
		background: hsl(var(--color-bg-inverse));
		color: hsl(var(--color-text-inverse));
		box-shadow: 0 2px 4px hsl(var(--color-neutral-100) / 0.15), 0 16px 32px -12px hsl(var(--color-neutral-100) / 0.45);
	}
	.hinweis a,
	.einrichtung a {
		font-weight: 700;
		text-decoration: underline;
		text-decoration-color: hsl(var(--color-green-100));
		text-decoration-thickness: 2px;
		text-underline-offset: 0.2em;
	}

	.einrichtung {
		position: fixed;
		left: 0;
		right: 0;
		bottom: 0;
		z-index: 9999;
		padding: 0.85rem 1.5rem 1rem;
		background: hsl(var(--color-bg-inverse));
		color: hsl(var(--color-text-inverse));
		box-shadow: 0 -8px 24px -12px hsl(var(--color-neutral-100) / 0.5);
		display: grid;
		gap: 0.6rem;
	}
	.einrichtung.fehler {
		background: hsl(var(--color-red-200));
	}
	.inhalt {
		display: flex;
		align-items: flex-start;
		gap: 0.75rem;
		max-width: 72rem;
	}
	.inhalt :global(.symbol) {
		flex: none;
		width: 1.5rem;
		height: 1.5rem;
		margin-top: 0.05rem;
	}
	.spur {
		height: 0.4rem;
		border-radius: 999px;
		background: hsl(var(--color-neutral-0) / 0.2);
		overflow: hidden;
	}
	.spur i {
		display: block;
		height: 100%;
		border-radius: inherit;
		background: hsl(var(--color-green-100));
		transition: width 0.3s ease-out;
	}

	.dialog {
		display: grid;
		gap: 1rem;
	}
	.lizenz {
		margin: 0;
		font-size: 0.875rem;
		color: hsl(var(--color-text-secondary));
	}
	.lizenz a {
		text-decoration: underline;
		text-underline-offset: 0.2em;
	}
	.dateien {
		margin: 0;
		padding: 0.6rem 0.9rem;
		max-height: 7rem;
		overflow: auto;
		list-style: none;
		border-radius: 0.5rem;
		background: hsl(var(--color-bg-secondary));
		color: hsl(var(--color-text-secondary));
		overflow-wrap: anywhere;
	}
	.sofort {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		min-height: var(--hit);
	}
	.sofort input {
		width: 1.25rem;
		height: 1.25rem;
	}
</style>

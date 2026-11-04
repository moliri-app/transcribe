<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Der Editor: der aus Moliri gespiegelte Transkript-Editor als ganze Seite, mit dem Speicher für die
     API der EXE ($lib/editor/speicher.js). Oben links die Wahl der Aufnahme; solange keine offen ist,
     steht an der Stelle des Editors, warum (wird geladen, noch keine, Fehler). -->
<script>
	import { onMount } from 'svelte';
	import { Editor } from '$lib/moliri/components/files/transcript/editor/editor.svelte.js';
	import TranscriptEditor from '$lib/moliri/components/files/transcript/editor/TranscriptEditor.svelte';
	import { loadRecording, merken, recordingName, transcribeStore } from '$lib/editor/speicher.js';

	/** @type {import('$lib/editor/speicher.js').Datei[]} */
	let files = $state.raw([]);
	/** @type {string | null} */ let stem = $state(null);
	/** @type {Editor | null} */ let ed = $state.raw(null);
	/** 'laden' | 'leer' (noch kein Ergebnis) | 'fehler' | 'bereit' */
	let phase = $state('laden');
	let error = $state('');
	let openSeq = 0;

	const file = $derived(files.find((f) => f.stem === stem) ?? null);
	const title = $derived(file ? `${recordingName(file)} · Editor · Transcribe by Moliri` : 'Editor · Transcribe by Moliri');

	onMount(async () => {
		try {
			/** @type {typeof files} */
			const idx = await (await fetch('/api/index')).json();
			files = idx.filter((f) => f.engines.some((e) => e.final));
		} catch {
			phase = 'fehler';
			error = 'Die Liste der Aufnahmen ließ sich nicht laden.';
			return;
		}
		if (!files.length) {
			phase = 'leer';
			return;
		}
		const saved = files.find((f) => f.stem === merken.get('stem'));
		await open((saved ?? files[0]).stem);
	});

	// Erst Transkript und Korrekturen laden, dann umschalten. Nur die jüngste Wahl schaltet um: eine
	// langsame frühere (lange Aufnahme), die nach einer späteren fertig wird, darf sie nicht ersetzen.
	async function open(/** @type {string} */ next) {
		const f = files.find((x) => x.stem === next);
		if (!f) return;
		// erst speichern, was zur bisherigen Aufnahme gehört; schlägt das fehl, bleibt sie offen
		if (ed && stem !== next && !(await ed.saveNow())) {
			alert('Die Korrekturen ließen sich nicht speichern. Die Aufnahme wurde nicht gewechselt.');
			return;
		}
		const seq = ++openSeq;
		let data;
		try {
			data = await loadRecording(f);
		} catch {
			if (seq !== openSeq) return;
			phase = 'fehler';
			error = `„${recordingName(f)}“ ließ sich nicht laden.`;
			ed = null;
			return;
		}
		if (seq !== openSeq) return;
		ed?.media?.pause();
		const next_ = new Editor(transcribeStore(next));
		next_.load(data);
		stem = next;
		merken.set('stem', next);
		ed = next_;
		phase = 'bereit';
	}
</script>

<svelte:head><title>{title}</title></svelte:head>

{#snippet wahl()}
	<select
		class="datei"
		aria-label="Aufnahme wählen"
		value={stem ?? ''}
		disabled={!files.length}
		onchange={async (e) => {
			const el = e.currentTarget;
			// dieselbe Aufnahme noch einmal gewählt: nicht neu laden, das würde Korrekturen der letzten
			// Augenblicke mit dem älteren Stand vom Server überschreiben
			if (el.value !== stem) await open(el.value);
			el.value = stem ?? ''; // die Liste nennt immer, was zu sehen ist
		}}
	>
		{#each files as f (f.stem)}<option value={f.stem}>{recordingName(f)}</option>{/each}
	</select>
{/snippet}

<div class="seite">
	{#if ed && phase === 'bereit'}
		{#key ed}
			<TranscriptEditor {ed} compact={false} title={wahl} />
		{/key}
	{:else}
		<header class="kopf">{@render wahl()}</header>
		<div class="leer" role="region" aria-label="Transkript">
			<div class="blatt">
				{#if phase === 'laden'}
					<p class="lade">Das Transkript wird geladen …</p>
				{:else if phase === 'leer'}
					<h1>Noch kein Transkript</h1>
					<p>
						Sobald eine Aufnahme transkribiert ist, steht sie hier zum Lesen und Abspielen bereit. Aufnahmen kommen über die
						<a href="/" data-sveltekit-reload>Bibliothek</a> dazu.
					</p>
				{:else}
					<h1>Das hat nicht geklappt</h1>
					<p>{error} Läuft Transcribe noch? Dann die Seite neu laden.</p>
				{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	.seite {
		height: calc(100dvh - 60px);
		display: flex;
		flex-direction: column;
		background: hsl(var(--color-bg-secondary));
		color: hsl(var(--color-text-primary));
	}

	/* die Wahl der Aufnahme ist der Titel des Editors */
	.datei {
		appearance: none;
		max-width: 100%;
		border: 0;
		border-radius: 0.5rem;
		padding: 0.125rem 2rem 0.125rem 0.25rem;
		margin-left: -0.25rem;
		background: transparent
			url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%231b1c1d' stroke-width='2' stroke-linecap='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E")
			no-repeat right 0.25rem center / 1.25rem;
		color: hsl(var(--color-text-primary));
		font: inherit;
		font-size: 1.375rem;
		font-weight: 700;
		letter-spacing: -0.01em;
		text-overflow: ellipsis;
		field-sizing: content;
		cursor: pointer;
	}
	.datei:hover {
		background-color: hsl(var(--color-bg-secondary));
	}
	.datei option {
		font-size: 1rem;
		font-weight: 400;
	}

	.kopf {
		display: flex;
		align-items: center;
		min-height: 3.75rem;
		padding: 0.5rem 1rem 0.5rem 1.25rem;
		background: hsl(var(--color-bg-primary));
		border-bottom: 1px solid hsl(var(--color-neutral-10));
	}
	.leer {
		flex: 1;
		min-height: 0;
		padding: 1.25rem 2rem;
		overflow-y: auto;
	}
	.blatt {
		max-width: 46rem;
		margin: 0 auto;
		background: hsl(var(--color-bg-primary));
		border-radius: 0.75rem;
		padding: 1.75rem 2.25rem 2rem;
	}
	.blatt h1 {
		font-size: 1.5rem;
		margin-bottom: 0.5rem;
	}
	.blatt p {
		font-size: 1.125rem;
		color: hsl(var(--color-text-secondary));
	}
	.blatt a {
		font-weight: 600;
		color: hsl(var(--color-text-primary));
		text-decoration: underline;
		text-underline-offset: 0.15em;
	}
	.lade {
		animation: atmen 1.6s ease-in-out infinite;
	}
	@keyframes atmen {
		50% {
			opacity: 0.5;
		}
	}
</style>

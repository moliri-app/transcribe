<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Programme verbinden: die Adresse der Live-API, die beiden Arten, sie zu nutzen, was von den Anfragen
     gespeichert bleibt, welche Webseiten sich verbinden dürfen, und für Entwickler das Protokoll. -->
<script>
	import Button from '$lib/moliri/components/ui/Button.svelte';
	import Toggle from '$lib/moliri/components/ui/Toggle.svelte';
	import { live, LOESCHEN } from './stand.svelte.js';

	/** @type {{ onzurueck?: () => void }} */
	let { onzurueck } = $props();

	const adresse = `ws://${typeof location === 'undefined' ? 'localhost:31765' : location.host}/api/live`;

	let kopiert = $state(false);
	let meldung = $state('');
	async function kopieren() {
		try {
			await navigator.clipboard.writeText(adresse);
			kopiert = true;
			setTimeout(() => (kopiert = false), 2000);
		} catch {
			meldung = 'Kopieren ging nicht.';
		}
	}
</script>

<article class="verbinden" aria-labelledby="h-verbinden">
	<header class="kopf">
		{#if onzurueck}
			<button class="zurueck" type="button" onclick={onzurueck}>
				<IconMaterialSymbolsArrowBackRounded aria-hidden="true" />Alle Aufnahmen
			</button>
		{/if}
		<h1 id="h-verbinden">Programme verbinden</h1>
		<p>Andere Programme, etwa Moliri, holen sich hier Sprache als Text und übernehmen ihn direkt in ein Feld, oder lassen den Text von Bildern lesen.</p>
	</header>

	<div class="tisch">
		{#if live.abgelehnt.length}
			<section class="blatt anfrage" aria-labelledby="h-abgelehnt">
				<h2 id="h-abgelehnt">Wollten sich verbinden</h2>
				<p class="meta">Diese Webseiten haben es versucht und wurden abgewiesen. Erlaube nur, was du kennst.</p>
				<ul class="seiten">
					{#each live.abgelehnt as a (a.origin)}
						<li>
							<span class="herkunft">{a.origin === 'null' ? 'eine Datei oder eingebettete Seite' : a.origin}</span>
							{#if a.client && a.client !== a.origin}<span class="meta">({a.client})</span>{/if}
							<Button type="primary" size="large" onclick={() => live.freigeben(a.origin, true)}>Erlauben</Button>
						</li>
					{/each}
				</ul>
			</section>
		{/if}

		<section class="blatt" aria-labelledby="h-adresse">
			<h2 id="h-adresse">Adresse</h2>
			{#if !live.an}
				<p class="aus" role="status">
					<IconMaterialSymbolsInfoOutlineRounded aria-hidden="true" />„Im Hintergrund bereit“ ist aus: Programme dürfen das
					Mikrofon nicht über Transcribe aufnehmen lassen, bis du es unten wieder einschaltest. Audiodaten und Bilder,
					die sie schicken, verarbeitet Transcribe trotzdem.
				</p>
			{/if}
			<p>Diese Adresse in das Programm eintragen, in Moliri beim Feld „API“:</p>
			<div class="adresse">
				<code>{adresse}</code>
				<Button type="secondary" size="large" onclick={kopieren}>
					{#snippet iconLeft()}<IconMaterialSymbolsContentCopyOutlineRounded />{/snippet}
					{kopiert ? 'Kopiert' : 'Adresse kopieren'}
				</Button>
			</div>
			{#if meldung}<p class="meta" role="status">{meldung}</p>{/if}
			<dl class="modi">
				<dt>Audio schicken</dt>
				<dd>Das Programm nimmt selbst auf und schickt den Ton; der Text kommt sofort zurück.</dd>
				<dt>Mithören lassen <code>?mode=mic</code></dt>
				<dd>
					Das Programm sagt nur „ab jetzt“ und „bis jetzt“. Dazwischen nimmt Transcribe auf, was unter „Einrichten“
					gewählt ist (Mikrofon, Systemaudio oder beides), und schickt den Text laufend an das Programm.
				</dd>
			</dl>
			<p class="meta">
				Jede Anfrage erscheint links als Aufnahme „Per API“. Das Programm Transcribe muss dafür laufen; solange es Text
				liefert, beendet es sich nicht. Was Programme lesen lassen, das Sprachmodell fragen oder für die Suche nach
				Bedeutung umwandeln, steht links unter „KI & OCR“: mit Anfrage, Antwort, der KI, die antwortete, und der Dauer.
			</p>
		</section>

		<section class="blatt" aria-labelledby="h-behalten">
			<h2 id="h-behalten">Was von den Anfragen bleibt</h2>
			<Toggle
				bind:checked={() => live.apiSpeichern, (v) => live.apiSpeichernSetzen(v)}
				title="Anfragen als Aufnahme speichern"
				description={live.apiSpeichern
					? 'Jede Anfrage bleibt links stehen, mit ihrem Text, auch nach einem Neustart.'
					: 'Aus: Eine Anfrage steht links nur, solange sie läuft, und ein paar Minuten danach. Auf dem Rechner bleibt nichts davon.'}
			/>
			{#if live.apiSpeichern}
				<label class="feldname">
					Automatisch löschen
					<select class="feld" value={live.apiLoeschen} onchange={(e) => live.apiLoeschenSetzen(Number(e.currentTarget.value))}>
						{#each LOESCHEN as [stunden, name] (stunden)}<option value={stunden}>{name}</option>{/each}
						{#if !LOESCHEN.some(([stunden]) => stunden === live.apiLoeschen)}
							<option value={live.apiLoeschen}>Nach {live.apiLoeschen} Stunden</option>
						{/if}
					</select>
					<span class="satz">
						Gezählt ab dem Ende einer Anfrage; das gilt auch für die, die schon gespeichert sind. Aufnahmen, die du hier
						startest, und Diktate bleiben.
					</span>
				</label>
			{/if}
		</section>

		<section class="blatt" aria-labelledby="h-erlaubt">
			<h2 id="h-erlaubt">Wer sich verbinden darf</h2>
			<p class="meta">
				Programme auf diesem Rechner dürfen sich immer verbinden. Webseiten von anderen Adressen erst, wenn du sie erlaubst;
				wer es versucht, erscheint hier oben. Die Erlaubnis gilt für Live-Transkription und Texterkennung.
			</p>
			{#if live.erlaubt.length}
				<ul class="seiten">
					{#each live.erlaubt as o (o)}
						<li>
							<span class="herkunft">{o}</span>
							<Button type="tertiary" size="large" onclick={() => live.freigeben(o, false)}>Nicht mehr erlauben</Button>
						</li>
					{/each}
				</ul>
			{:else}
				<p class="leer">Noch keine Webseite erlaubt.</p>
			{/if}
		</section>

		<section class="blatt" aria-labelledby="h-protokoll">
			<details>
				<summary><h2 id="h-protokoll">Für Entwickler: so spricht man mit Transcribe</h2></summary>
				<div class="protokoll">
					<p>
						WebSocket auf <code>{adresse}</code>. Parameter: <code>mode=audio</code> (Standard) oder <code>mic</code>,
						<code>name=</code> wie die Aufnahme heißen soll, bei Audio <code>sample_rate=16000</code> und
						<code>encoding=s16le</code> oder <code>f32le</code> (Mono).
					</p>
					<p>
						Audio geht als binäre Nachrichten hin. Zurück kommen JSON-Nachrichten: <code>ready</code>, laufend
						<code>partial</code> (der Satz, der gerade gesprochen wird), <code>final</code> (fertiger Satz, mit
						<code>start</code> und <code>end</code> in Sekunden), am Ende <code>done</code>, bei Problemen
						<code>error</code>. <code>{'{"type":"stop"}'}</code> beendet: Transcribe schickt noch, was bis jetzt gesagt
						wurde, dann <code>done</code>.
					</p>
					<pre>{`const ws = new WebSocket('${adresse}?mode=mic&name=Moliri');
ws.onmessage = (e) => {
  const m = JSON.parse(e.data);
  if (m.type === 'partial') feld.placeholder = m.text;
  if (m.type === 'final') feld.value += m.text + ' ';
};
// später: ws.send(JSON.stringify({ type: 'stop' }));`}</pre>
				</div>
			</details>
		</section>
	</div>
</article>

<style>
	.verbinden {
		display: grid;
		grid-template-rows: auto minmax(0, 1fr);
		min-height: 0;
		min-width: 0;
	}
	.kopf {
		display: grid;
		gap: 0.25rem;
		padding: 0.875rem 1.5rem 1rem 1.75rem;
		background: hsl(var(--color-bg-primary));
		border-bottom: 1px solid hsl(var(--color-border-primary));
	}
	h1 {
		font-size: 1.375rem;
		letter-spacing: -0.01em;
	}
	.kopf p {
		max-width: 62ch;
		color: hsl(var(--color-text-secondary));
	}
	.zurueck {
		justify-self: start;
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		min-height: var(--hit);
		margin: -0.5rem 0 0 -0.5rem;
		padding: 0 0.75rem 0 0.5rem;
		border-radius: 2rem;
		font-weight: 600;
		cursor: pointer;
	}
	.zurueck:hover {
		background: hsl(var(--color-bg-secondary));
	}

	.tisch {
		overflow-y: auto;
		padding: 1.25rem 1.5rem 2rem;
		display: grid;
		gap: 1.25rem;
		align-content: start;
		justify-items: center;
	}
	.blatt {
		width: 100%;
		max-width: 52rem;
		display: grid;
		gap: 1rem;
		padding: 1.5rem 1.75rem 1.75rem;
		background: hsl(var(--color-bg-primary));
		border-radius: 0.75rem;
		box-shadow:
			0 1px 2px hsl(var(--color-neutral-100) / 0.05),
			0 12px 32px -20px hsl(var(--color-neutral-100) / 0.25);
	}
	/* wer sich verbinden wollte: ein Signal-Hauch, damit es auffällt */
	.blatt.anfrage {
		background: hsl(var(--color-green-10));
	}
	h2 {
		font-size: 1.25rem;
	}
	.meta,
	.leer {
		max-width: 62ch;
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
	}

	.adresse {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem 1rem;
	}
	code,
	pre {
		font-family: var(--font-mono);
		font-size: 0.9375em;
	}
	.adresse code {
		padding: 0.55rem 0.9rem;
		border-radius: 0.5rem;
		background: hsl(var(--color-bg-secondary));
		font-size: 1.0625rem;
		overflow-wrap: anywhere;
	}
	.modi {
		display: grid;
		grid-template-columns: 12rem minmax(0, 1fr);
		gap: 0.75rem 1.5rem;
		max-width: 52rem;
		margin: 0.25rem 0;
		padding: 1rem 0;
		border-block: 1px solid hsl(var(--color-border-primary));
	}
	.modi dt {
		font-weight: 700;
	}
	.modi dt code {
		display: block;
		font-weight: 400;
		color: hsl(var(--color-text-secondary));
	}
	.modi dd {
		margin: 0;
		max-width: 62ch;
	}

	.feldname {
		max-width: 28rem;
	}
	.feldname select {
		width: 100%;
		font-weight: 400;
	}
	.satz {
		font-size: 0.9375rem;
		font-weight: 400;
	}

	.seiten {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
	}
	.seiten li {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.25rem 0.75rem;
		padding: 0.5rem 0;
		border-top: 1px solid hsl(var(--color-border-primary));
	}
	.seiten li:first-child {
		border-top: 0;
	}
	.herkunft {
		flex: 1;
		font-weight: 700;
		overflow-wrap: anywhere;
	}

	summary {
		min-height: var(--hit);
	}
	summary h2 {
		display: inline;
	}
	.protokoll {
		display: grid;
		gap: 0.75rem;
		max-width: 70ch;
		margin-top: 0.75rem;
	}
	.protokoll pre {
		padding: 0.75rem 1rem;
		border-radius: 0.5rem;
		background: hsl(var(--color-bg-secondary));
		font-size: 0.875rem;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}

	@media (max-width: 720px) {
		.kopf {
			padding: 0.75rem 1rem;
		}
		.tisch {
			padding: 0.75rem 0.5rem 1.5rem;
		}
		.blatt {
			padding: 1.25rem 1rem;
		}
		.modi {
			grid-template-columns: minmax(0, 1fr);
			gap: 0.25rem;
		}
		.modi dd {
			margin-bottom: 0.75rem;
		}
	}
	.aus {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
		padding: 0.625rem 0.875rem;
		border-radius: 0.5rem;
		background: hsl(var(--color-bg-secondary));
		font-weight: 600;
	}
	.aus :global(svg) {
		flex: none;
		width: 1.25rem;
		height: 1.25rem;
		margin-top: 0.125rem;
	}
</style>

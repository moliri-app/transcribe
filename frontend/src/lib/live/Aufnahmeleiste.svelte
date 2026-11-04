<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Aufnahmeleiste der Live-Seite: Tintenleiste am unteren Rand wie die Player-Leiste des Editors.
     Links Aufnehmen und Beenden mit der Laufzeit, in der Mitte die Pegel, rechts der Schalter
     „Im Hintergrund bereit“ (Programme dürfen das Mikrofon aufnehmen lassen, die Erkennung bleibt
     geladen; geschickte Audiodaten und Bilder und von Hand aufnehmen gehen immer) und „Einrichten“: Quelle, Mikrofon und Speichern, daneben „Meine Stimme“
     mit Sprache, eigenen Wörtern und angesagten Satzzeichen, stehen in einem eigenen Fenster über der
     Leiste, nicht im Weg. -->
<script>
	import { tick } from 'svelte';
	import { dauer } from '$lib/api.js';
	import Sprachenwahl from '$lib/Sprachenwahl.svelte';
	import Toggle from '$lib/moliri/components/ui/Toggle.svelte';
	import MeineWoerter from './MeineWoerter.svelte';
	import { EINGANG, laenge, live, QUELLEN, sprachname } from './stand.svelte.js';

	let offen = $state(false);
	/** @type {HTMLElement | undefined} */
	let fenster = $state();
	/** @type {HTMLButtonElement | undefined} */
	let knopf = $state();

	/** das gewählte Mikrofon ist gerade nicht angeschlossen */
	const fehlt = $derived(!!live.geraet && !!live.geraete && !live.geraete.includes(live.geraet));

	const quelle = $derived((live.mic.running && live.mic.source) || live.quelle);
	/** die Pegel, die zur laufenden Aufnahme gehören, sonst zur gewählten Quelle */
	const pegelQuellen = $derived(/** @type {const} */ ({ mic: ['mic'], system: ['system'], both: ['mic', 'system'] })[quelle]);

	/** was eingestellt ist, in einer Zeile */
	const stand = $derived(
		[QUELLEN[live.quelle], sprachname(live.sprache), live.speichern ? 'wird gespeichert' : 'nur Text'].join(' · ')
	);

	/** was der Schalter „Im Hintergrund bereit“ sagt */
	const modus = $derived(
		!live.an ? 'aus' : live.warm === 'ready' ? 'an, sofort bereit' : live.warm === 'loading' ? 'lädt die Erkennung …' : 'an'
	);

	/** so lange wartet die Aufnahme schon auf den ersten Ton, in Sekunden */
	const wartet = $derived(
		live.laufend?.waiting ? (live.jetzt - new Date(live.laufend.started).getTime()) / 1000 : 0
	);
	/** wie das Mikrofon heißt, auf das gewartet wird */
	const mikrofon = $derived(live.geraet || live.standard || 'dem Mikrofon');

	$effect(() => {
		if (offen) tick().then(() => fenster?.querySelector('input, select')?.focus?.());
	});

	function schliessen() {
		offen = false;
		knopf?.focus();
	}

	/** Klick neben das Fenster schließt es
	 * @param {PointerEvent} e */
	function daneben(e) {
		const t = /** @type {Node} */ (e.target);
		if (offen && !fenster?.contains(t) && !knopf?.contains(t)) offen = false;
	}
</script>

<svelte:window onpointerdown={daneben} />

<div class="leiste" role="group" aria-label="Aufnehmen">
	<div class="links">
		{#if live.laufend}
			<button class="beenden" type="button" onclick={() => live.beenden()}>
				<IconMaterialSymbolsStopRounded aria-hidden="true" />Beenden
			</button>
			<div class="jetzt">
				{#if live.laufend.loading || live.laufend.waiting}
					<span class="uhr wartet">startet …</span>
				{:else}
					<span class="uhr"><span class="punkt" aria-hidden="true"></span>{dauer(laenge(live.laufend, live.jetzt))}</span>
				{/if}
				<span class="unter">
					{QUELLEN[live.mic.source ?? 'mic']}{#if live.laufend.audio}&nbsp;· wird gespeichert{/if}
				</span>
			</div>
		{:else}
			<button class="starten" type="button" disabled={live.startet} aria-busy={live.startet} onclick={() => live.starten()}>
				<span class="kreis">
					{#if live.startet}<IconEosIconsLoading aria-hidden="true" />{:else}<IconMaterialSymbolsMicRounded aria-hidden="true" />{/if}
				</span>
				<span class="jetzt">
					<span class="was">{live.startet ? 'Startet …' : 'Aufnahme starten'}</span>
					<span class="unter">{stand}</span>
				</span>
			</button>
		{/if}
	</div>

	<p class="meldung" role="status">
		{#if live.meldung}{live.meldung}{:else if wartet > 6 && live.quelle !== 'system'}<span class="eingang"
				><IconMaterialSymbolsInfoOutlineRounded aria-hidden="true" /><span
					>Von {mikrofon} kommt noch kein Ton. Ist es eingeschaltet und verbunden?
					<button class="link" type="button" onclick={() => (offen = true)}>Ein anderes Mikrofon wählen?</button></span
				></span
			>{:else if live.mic.running && live.mic.input}<span class="eingang"
				><IconMaterialSymbolsInfoOutlineRounded aria-hidden="true" /><span
					>{EINGANG[live.mic.input]}
					<button class="link" type="button" onclick={() => (offen = true)}>Ein anderes Mikrofon wählen?</button></span
				></span
			>{:else if live.mic.listeners}{live.mic.listeners === 1
				? 'Ein Programm hört mit.'
				: `${live.mic.listeners} Programme hören mit.`}{/if}
	</p>

	<div class="pegel" class:an={live.mic.running} aria-hidden="true">
		{#each pegelQuellen as q (q)}
			<div class="balken">
				<span>{q === 'mic' ? 'Mikrofon' : 'System'}</span>
				<i><b style:width="{Math.round((live.pegel[q] ?? 0) * 100)}%"></b></i>
			</div>
		{/each}
	</div>

	<button
		class="modus"
		type="button"
		role="switch"
		aria-checked={live.an}
		aria-label="Im Hintergrund bereit"
		title={live.an
			? 'An: Andere Programme wie Moliri dürfen über Transcribe das Mikrofon aufnehmen lassen, und die Erkennung bleibt geladen, damit alles sofort startet. Das belegt etwa 1 GB Arbeitsspeicher. Audiodaten und Bilder, die ein Programm schickt, verarbeitet Transcribe immer.'
			: 'Aus: Andere Programme dürfen das Mikrofon nicht über Transcribe aufnehmen lassen, und nichts bleibt geladen. Audiodaten und Bilder, die ein Programm schickt, verarbeitet Transcribe trotzdem. Von Hand aufnehmen geht auch, es startet nur etwas langsamer.'}
		onclick={() => live.modusSetzen(!live.an)}
	>
		<span class="schiene" aria-hidden="true"><span class="knubbel"></span></span>
		<span class="mtext"><span class="mname">Im Hintergrund bereit</span><span class="mstand">{modus}</span></span>
	</button>

	<button
		bind:this={knopf}
		class="einrichten"
		type="button"
		aria-expanded={offen}
		aria-controls="einrichten"
		onclick={() => (offen = !offen)}
	>
		<IconMaterialSymbolsTuneRounded aria-hidden="true" /><span class="lbl">Einrichten</span>
	</button>

	{#if offen}
		<div
			id="einrichten"
			class="fenster"
			bind:this={fenster}
			aria-labelledby="h-einrichten"
			role="dialog"
			tabindex="-1"
			onkeydown={(e) => e.key === 'Escape' && (e.stopPropagation(), schliessen())}
		>
			<div class="fkopf">
				<h2 id="h-einrichten">Aufnahme einrichten</h2>
				<button class="zu" type="button" onclick={schliessen}>
					<IconMaterialSymbolsCloseRounded aria-hidden="true" /><span class="sr">Schließen</span>
				</button>
			</div>
			{#if live.mic.running}<p class="hinweis">Änderungen gelten ab der nächsten Aufnahme.</p>{/if}

			<section class="teil" aria-labelledby="h-aufnahme">
				<h3 id="h-aufnahme">Aufnahme</h3>
				<fieldset class="quellen">
					<legend class="feldname">Was aufgenommen wird</legend>
					{#each /** @type {const} */ ([['mic', 'Mikrofon', 'Was am Mikrofon gesagt wird.'], ['system', 'Systemaudio', 'Was der Rechner abspielt, etwa die anderen in einem Online-Meeting.'], ['both', 'Beides gemischt', 'Am besten mit Kopfhörern, sonst sind die anderen doppelt zu hören.']]) as [wert, name, satz] (wert)}
						{@const aus = wert !== 'mic' && !live.systemMoeglich}
						<label class="quelle" class:an={live.quelle === wert} class:aus>
							<input type="radio" name="quelle" value={wert} disabled={aus} bind:group={live.quelle} onchange={live.quelleWaehlen} />
							<span>
								<b>{name}</b>
								<span class="satz">{aus ? 'Auf diesem Rechner nicht möglich.' : satz}</span>
							</span>
						</label>
					{/each}
				</fieldset>

				{#if live.quelle !== 'system'}
					<label class="feldname">
						Mikrofon
						<select class="feld" bind:value={live.geraet} onchange={live.geraetWaehlen} disabled={live.geraete === null}>
							<option value="">
								{live.geraete === null ? 'wird gesucht …' : live.standard ? `Systemstandard: ${live.standard}` : 'Systemstandard'}
							</option>
							{#if fehlt}
								<option value={live.geraet}>{live.geraet} (nicht angeschlossen)</option>
							{/if}
							{#each live.geraete ?? [] as g (g)}<option value={g}>{g}</option>{/each}
						</select>
						<span class="satz">
							{#if fehlt}
								Dieses Mikrofon ist gerade nicht angeschlossen.
							{:else if live.geraet}
								Bleibt bei diesem Mikrofon, auch wenn ein anderes angeschlossen wird.
							{:else if live.standard}
								Nimmt das Mikrofon, das in Windows als Standard eingestellt ist. Wird ein anderes zum Standard, etwa ein
								angeschlossenes Headset, wechselt die Aufnahme mit.
							{:else}
								Nimmt das Mikrofon, das im System als Standard eingestellt ist.
							{/if}
						</span>
					</label>
				{/if}

				<Toggle
					bind:checked={() => live.speichern, (v) => live.speichernSetzen(v)}
					disabled={!!live.laufend}
					title="Aufnahme speichern"
					description={live.speichern
						? 'Sie kommt zu deinen Aufnahmen und lässt sich danach vollständig transkribieren, mit Sprechern.'
						: 'Aus: Es bleibt nur der Text.'}
				/>
			</section>

			<section class="teil stimme" aria-labelledby="h-stimme">
				<h3 id="h-stimme">Meine Stimme</h3>
				<div class="sprache">
					<Sprachenwahl bind:value={live.sprache} label="Sprachen" onchange={live.spracheWaehlen} />
					<span class="satz">
						{live.sprache === 'de'
							? 'Beim Sprechen zeigt das schnelle deutsche Modell Kroko den Text; jeden Satz schreibt danach das genauere Parakeet.'
							: live.sprache === 'auto'
								? 'Der Live-Text erkennt die Sprache selbst.'
								: live.sprache.includes(',')
									? 'Der Live-Text erkennt die Sprache selbst. Vollständig transkribiert bekommt jeder Abschnitt eine dieser Sprachen.'
									: 'Der Live-Text erkennt die Sprache selbst; die Wahl gilt fürs vollständige Transkribieren.'}
					</span>
				</div>
				<MeineWoerter />
				<Toggle
					bind:checked={() => live.ansagen, (v) => live.ansagenSetzen(v)}
					title="Satzzeichen ansagen"
					description={live.ansagen
						? 'Sag „Komma“, „Punkt“, „Fragezeichen“ oder „neuer Absatz“, und das Zeichen steht da. Sonst setzt die App sie selbst.'
						: 'Aus: Die App setzt alle Satzzeichen selbst, „Komma“ bleibt ein Wort.'}
				/>
			</section>
		</div>
	{/if}
</div>

<style>
	/* Tinte wie die Player-Leiste des Editors */
	.leiste {
		position: relative;
		z-index: 5;
		display: flex;
		align-items: center;
		gap: 1rem 1.5rem;
		min-height: 4.5rem;
		padding: 0.625rem 1rem 0.625rem 1.25rem;
		background: hsl(var(--color-neutral-100));
		color: hsl(var(--color-neutral-0));
	}
	.leiste :global(:focus-visible) {
		outline-color: hsl(var(--color-green-100));
	}
	.links {
		display: flex;
		align-items: center;
		gap: 1rem;
		min-width: 0;
	}

	/* Aufnehmen: der Signalkreis wie der Abspielknopf, die Beschriftung gehört zum Knopf */
	.starten {
		display: flex;
		align-items: center;
		gap: 0.875rem;
		min-width: 0;
		padding: 0.25rem 1.25rem 0.25rem 0.25rem;
		border-radius: 2rem;
		text-align: left;
		cursor: pointer;
		transition: background-color 0.15s;
	}
	.starten:hover:not(:disabled) {
		background: hsl(var(--color-neutral-0) / 0.08);
	}
	.starten:disabled {
		cursor: progress;
	}
	.kreis {
		flex: none;
		width: 3.25rem;
		height: 3.25rem;
		display: grid;
		place-items: center;
		border-radius: 50%;
		background: hsl(var(--color-green-100));
		color: hsl(var(--color-neutral-100));
		transition: transform 0.15s;
	}
	.starten:hover:not(:disabled) .kreis {
		transform: scale(1.05);
	}
	.kreis :global(svg) {
		width: 1.75rem;
		height: 1.75rem;
	}
	.jetzt {
		display: flex;
		flex-direction: column;
		gap: 0.0625rem;
		min-width: 0;
	}
	.was {
		font-weight: 700;
		font-size: 1.0625rem;
	}
	.unter {
		font-size: 0.9375rem;
		color: hsl(var(--color-neutral-10));
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* Beenden: Papierpille, daneben die Laufzeit mit atmendem Signalpunkt */
	.beenden {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		min-height: 3.25rem;
		padding: 0 1.375rem 0 1rem;
		border-radius: 2rem;
		background: hsl(var(--color-neutral-0));
		color: hsl(var(--color-neutral-100));
		font-weight: 700;
		font-size: 1.0625rem;
		cursor: pointer;
		transition: background-color 0.15s;
	}
	.beenden:hover {
		background: hsl(var(--color-neutral-10));
	}
	.beenden :global(svg) {
		width: 1.625rem;
		height: 1.625rem;
	}
	.uhr {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 1.375rem;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	.uhr.wartet {
		font-size: 1.0625rem;
		color: hsl(var(--color-neutral-10));
	}
	.punkt {
		width: 0.625rem;
		height: 0.625rem;
		border-radius: 50%;
		background: hsl(var(--color-green-100));
		animation: atmen 1.2s ease-in-out infinite;
	}
	@keyframes atmen {
		50% {
			opacity: 0.3;
		}
	}

	.meldung {
		flex: 1;
		min-width: 0;
		font-size: 0.9375rem;
		color: hsl(var(--color-neutral-10));
	}

	/* was mit dem Mikrofon nicht stimmt: ruhig, ohne Alarmfarbe */
	.eingang {
		display: inline-flex;
		align-items: flex-start;
		gap: 0.5rem;
		line-height: 1.4;
	}
	.eingang :global(svg) {
		flex: none;
		width: 1.25rem;
		height: 1.25rem;
		margin-top: 0.0625rem;
	}
	.link {
		padding: 0;
		border: 0;
		background: none;
		color: inherit;
		font: inherit;
		font-weight: 600;
		text-decoration: underline;
		text-underline-offset: 0.2em;
		cursor: pointer;
	}

	/* Pegel: wie laut Mikrofon und Systemaudio gerade sind, in Signalgrün */
	.pegel {
		flex: 0 1 15rem;
		display: grid;
		gap: 0.375rem;
		opacity: 0.4;
		transition: opacity 0.2s;
	}
	.pegel.an {
		opacity: 1;
	}
	/* Im Hintergrund bereit: Schalter mit Signalgrün, darunter ob die Erkennung bereitsteht */
	.modus {
		flex: none;
		display: inline-flex;
		align-items: center;
		gap: 0.625rem;
		min-height: var(--hit);
		padding: 0.25rem 0.75rem 0.25rem 0.5rem;
		border-radius: 2rem;
		text-align: left;
		cursor: pointer;
		transition: background-color 0.15s;
	}
	.modus:hover {
		background: hsl(var(--color-neutral-0) / 0.08);
	}
	.schiene {
		flex: none;
		position: relative;
		width: 2.5rem;
		height: 1.5rem;
		border-radius: 2rem;
		background: hsl(var(--color-neutral-0) / 0.22);
		transition: background-color 0.15s;
	}
	.knubbel {
		position: absolute;
		top: 0.1875rem;
		left: 0.1875rem;
		width: 1.125rem;
		height: 1.125rem;
		border-radius: 50%;
		background: hsl(var(--color-neutral-0));
		transition: translate 0.15s;
	}
	.modus[aria-checked='true'] .schiene {
		background: hsl(var(--color-green-100));
	}
	.modus[aria-checked='true'] .knubbel {
		translate: 1rem 0;
		background: hsl(var(--color-neutral-100));
	}
	.mtext {
		display: flex;
		flex-direction: column;
		line-height: 1.25;
	}
	.mname {
		font-weight: 600;
	}
	.mstand {
		font-size: 0.8125rem;
		color: hsl(var(--color-neutral-10));
		white-space: nowrap;
	}
	.balken {
		display: grid;
		grid-template-columns: 4.75rem 1fr;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.8125rem;
		font-weight: 600;
		letter-spacing: 0.04em;
		color: hsl(var(--color-neutral-10));
	}
	.balken i {
		display: block;
		height: 0.5rem;
		border-radius: 2rem;
		background: hsl(var(--color-neutral-0) / 0.16);
		overflow: hidden;
	}
	.balken b {
		display: block;
		height: 100%;
		border-radius: inherit;
		background: hsl(var(--color-green-100));
		transition: width 0.18s linear;
	}

	/* Einrichten: wie die Tempo-Pille der Player-Leiste */
	.einrichten {
		flex: none;
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		min-height: var(--hit);
		padding: 0 1.125rem 0 0.875rem;
		border: 1px solid hsl(var(--color-neutral-0) / 0.22);
		border-radius: 2rem;
		font-weight: 600;
		cursor: pointer;
		transition: background-color 0.15s;
	}
	.einrichten:hover {
		background: hsl(var(--color-neutral-0) / 0.1);
	}
	.einrichten[aria-expanded='true'] {
		background: hsl(var(--color-neutral-0));
		color: hsl(var(--color-neutral-100));
	}
	.einrichten :global(svg) {
		width: 1.375rem;
		height: 1.375rem;
	}

	/* Das Fenster: Papier über der Leiste, rechts, wie Hilfe und Sprechermenü im Editor */
	.fenster {
		position: absolute;
		right: 0.75rem;
		bottom: calc(100% + 0.625rem);
		width: min(27rem, calc(100vw - 1.5rem));
		max-height: calc(100dvh - 60px - 6rem);
		overflow-y: auto;
		display: grid;
		gap: 1.125rem;
		padding: 1.125rem 1.25rem 1.25rem;
		border-radius: 1rem;
		background: hsl(var(--color-bg-primary));
		color: hsl(var(--color-text-primary));
		box-shadow:
			0 1px 2px hsl(var(--color-neutral-100) / 0.08),
			0 14px 30px -10px hsl(var(--color-neutral-100) / 0.3);
		animation: auf 0.18s cubic-bezier(0.16, 1, 0.3, 1);
	}
	.fenster :global(:focus-visible) {
		outline-color: hsl(var(--color-border-focus));
	}
	@keyframes auf {
		from {
			opacity: 0;
			translate: 0 0.5rem;
		}
	}
	.fkopf {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: -0.375rem;
	}
	h2 {
		font-size: 1.25rem;
	}
	.zu {
		width: var(--hit);
		height: var(--hit);
		margin: -0.5rem -0.5rem -0.5rem 0;
		display: grid;
		place-items: center;
		border-radius: 50%;
		cursor: pointer;
	}
	.zu:hover {
		background: hsl(var(--color-bg-secondary));
	}
	.zu :global(svg) {
		width: 1.5rem;
		height: 1.5rem;
	}
	/* Aufnahme und Meine Stimme: untereinander, auf breitem Schirm nebeneinander */
	.teil {
		display: grid;
		align-content: start;
		gap: 1.125rem;
	}
	h3 {
		font-size: 0.875rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: hsl(var(--color-text-secondary));
		margin-bottom: -0.375rem;
	}
	.stimme {
		padding-top: 1.125rem;
		border-top: 1px solid hsl(var(--color-border-primary));
	}
	@media (min-width: 1000px) {
		.fenster {
			width: 54rem;
			grid-template-columns: 1fr 1fr;
			column-gap: 2rem;
		}
		.fkopf,
		.hinweis {
			grid-column: 1 / -1;
		}
		.stimme {
			padding-top: 0;
			padding-left: 2rem;
			border-top: 0;
			border-left: 1px solid hsl(var(--color-border-primary));
		}
	}
	.hinweis {
		padding: 0.5rem 0.75rem;
		border-radius: 0.5rem;
		background: hsl(var(--color-green-10));
		font-size: 0.9375rem;
	}

	.quellen {
		display: grid;
		gap: 0.375rem;
		border: 0;
		margin: 0;
		padding: 0;
	}
	.quellen legend {
		margin-bottom: 0.375rem;
	}
	.quelle {
		display: flex;
		align-items: flex-start;
		gap: 0.75rem;
		padding: 0.625rem 0.75rem;
		border: 1px solid hsl(var(--color-border-primary));
		border-radius: 0.75rem;
		cursor: pointer;
		transition:
			background-color 0.15s,
			border-color 0.15s;
	}
	.quelle:hover:not(.aus) {
		border-color: hsl(var(--color-text-secondary));
	}
	.quelle.an {
		border-color: hsl(var(--color-neutral-100));
		box-shadow: inset 0 0 0 1px hsl(var(--color-neutral-100));
	}
	.quelle.aus {
		cursor: default;
		opacity: 0.55;
	}
	.quelle input {
		margin-top: 0.1875rem;
	}
	.quelle > span {
		display: grid;
	}
	.sprache {
		display: grid;
		gap: 0.3rem;
	}
	.sprache :global(.sprachen) {
		width: 100%;
	}
	.satz {
		font-size: 0.9375rem;
		font-weight: 400;
		color: hsl(var(--color-text-secondary));
	}
	.feldname select {
		width: 100%;
		color: hsl(var(--color-text-primary));
		font-weight: 400;
	}

	@media (prefers-reduced-motion: reduce) {
		.punkt {
			animation: none;
		}
	}

	@media (max-width: 900px) {
		.meldung {
			display: none;
		}
		.pegel {
			flex: 1 1 6rem;
		}
	}
	@media (max-width: 560px) {
		.leiste {
			gap: 0.75rem;
			padding-inline: 0.5rem 0.75rem;
		}
		.unter,
		.pegel,
		.mtext {
			display: none;
		}
		.starten {
			margin-right: auto;
		}
		.links {
			flex: 1;
		}
		.einrichten .lbl {
			position: absolute;
			width: 1px;
			height: 1px;
			overflow: hidden;
			clip: rect(0 0 0 0);
		}
		.einrichten {
			padding: 0;
			width: var(--hit);
			justify-content: center;
		}
	}
</style>

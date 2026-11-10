<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Fragt auf jeder Seite, wenn eine Webseite Transcribe nutzen möchte und noch nicht darf: Erlauben
     oder Ablehnen, gleich hier (meist hat man es eben selbst versucht). Ist das Fenster zu oder nicht
     zu sehen, sagt es die EXE mit einer Benachrichtigung am Symbol (cmd/transcribe/main.go). Wollte
     ein Programm Transcribe nutzen, solange es nicht mit Windows startet, schlägt die Karte vor, das
     und das Weiterlaufen im Hintergrund einzuschalten, bis man es ablehnt. Den Stand liefert
     /api/access (internal/server/background.go). -->
<script>
	import { onMount } from 'svelte';
	import Button from '$lib/moliri/components/ui/Button.svelte';
	import { post } from '$lib/api.js';

	/** @typedef {{ origin: string, client?: string, at: string }} Frage */
	/** @type {{ blocked: Frage[], program: string, hint: boolean } | null} */
	let stand = $state(null);
	/** gerade erlaubt: die Karte sagt kurz, wie es weitergeht */
	let erlaubt = $state(/** @type {Frage | null} */ (null));
	/** gerade eingeschaltet: die Karte sagt es kurz */
	let eingeschaltet = $state(false);
	let fehler = $state('');
	let tut = $state('');

	const frage = $derived(stand?.blocked[0] ?? null);
	const weitere = $derived(Math.max(0, (stand?.blocked.length ?? 0) - 1));
	const name = (/** @type {Frage} */ f) => (f.origin === 'null' ? 'Eine Datei oder eingebettete Seite' : f.origin.replace(/^https?:\/\//, ''));

	async function laden() {
		try {
			const r = await fetch('/api/access');
			if (r.ok) stand = await r.json();
		} catch {
			// Server gerade nicht erreichbar: die Karte bleibt, wie sie ist
		}
	}

	/** @param {Frage} f @param {boolean} ja */
	async function antworten(f, ja) {
		fehler = '';
		tut = ja ? 'erlauben' : 'ablehnen';
		try {
			await post(ja ? '/api/live/allow' : '/api/live/forbid', { origin: f.origin });
			if (ja) {
				erlaubt = f;
				setTimeout(() => erlaubt === f && (erlaubt = null), 8000);
			}
			await laden();
		} catch (e) {
			fehler = /** @type {Error} */ (e).message;
		} finally {
			tut = '';
		}
	}

	/** @param {boolean} ja Mit Windows starten und weiterlaufen; nein: nicht mehr vorschlagen */
	async function hintergrund(ja) {
		fehler = '';
		tut = ja ? 'an' : 'nein';
		try {
			if (ja) {
				await post('/api/background', { autostart: true, background: true });
				eingeschaltet = true;
				setTimeout(() => (eingeschaltet = false), 6000);
			} else await post('/api/access', { hint: false });
			await laden();
		} catch (e) {
			fehler = /** @type {Error} */ (e).message;
		} finally {
			tut = '';
		}
	}

	onMount(() => {
		laden();
		const t = setInterval(laden, 2000);
		return () => clearInterval(t);
	});
</script>

{#if frage || erlaubt || stand?.hint || eingeschaltet}
	<section class="zugriff" aria-labelledby="h-zugriff-frage" aria-live="polite">
		{#if frage}
			<div class="teil">
				<h2 id="h-zugriff-frage"><IconMaterialSymbolsCableRounded aria-hidden="true" />Darf {name(frage)} Transcribe nutzen?</h2>
				<p>
					{#if frage.client && frage.client !== name(frage)}„{frage.client}“ auf dieser Webseite{:else}Diese Webseite{/if}
					möchte Sprache als Text holen oder Text aus Bildern lesen lassen. Erlaube nur, was du kennst.
				</p>
				<div class="knoepfe">
					<Button type="primary" size="large" loading={tut === 'erlauben'} onclick={() => antworten(frage, true)}>Erlauben</Button>
					<Button type="secondary" size="large" loading={tut === 'ablehnen'} onclick={() => antworten(frage, false)}>Ablehnen</Button>
				</div>
				{#if weitere}
					<p class="klein">
						{weitere === 1 ? 'Eine weitere Seite fragt' : `${weitere} weitere Seiten fragen`}, danach hier oder unter
						<a href="/settings#zugriff" data-sveltekit-reload>Einstellungen › Programme</a>.
					</p>
				{/if}
			</div>
		{:else if erlaubt}
			<div class="teil">
				<h2 id="h-zugriff-frage"><IconMaterialSymbolsCheckRounded aria-hidden="true" />{name(erlaubt)} darf Transcribe nutzen</h2>
				<p>Versuche es dort noch einmal. Zurücknehmen lässt es sich unter <a href="/settings#zugriff" data-sveltekit-reload>Einstellungen › Programme</a>.</p>
			</div>
		{/if}

		{#if stand?.hint}
			<div class="teil" class:darunter={frage || erlaubt}>
				{#if !frage && !erlaubt}
					<h2 id="h-zugriff-frage"><IconMaterialSymbolsCableRounded aria-hidden="true" />{stand.program} nutzt Transcribe</h2>
				{:else}
					<h3>Damit Programme Transcribe immer erreichen</h3>
				{/if}
				<p>
					Programme erreichen Transcribe nur, solange es läuft. Soll es mit Windows starten und nach dem Schließen im
					Infobereich der Taskleiste weiterlaufen?
				</p>
				<div class="knoepfe">
					<Button type={frage ? 'secondary' : 'primary'} size="large" loading={tut === 'an'} onclick={() => hintergrund(true)}>Einschalten</Button>
					<Button type="tertiary" size="large" loading={tut === 'nein'} onclick={() => hintergrund(false)}>Nein, danke</Button>
				</div>
			</div>
		{:else if eingeschaltet}
			<div class="teil" class:darunter={frage || erlaubt}>
				<p role="status">
					Transcribe startet jetzt mit Windows und läuft im Hintergrund weiter. Ändern lässt es sich unter
					<a href="/settings#bereit" data-sveltekit-reload>Einstellungen › Programme</a>.
				</p>
			</div>
		{/if}

		{#if fehler}<p class="fehler">{fehler}</p>{/if}
	</section>
{/if}

<style>
	/* Papier über der Seite, oben rechts unter der App-Leiste, wie das Fenster „Einrichten“ */
	.zugriff {
		position: fixed;
		top: calc(60px + 0.75rem);
		right: 0.75rem;
		z-index: 40;
		width: min(26rem, calc(100vw - 1.5rem));
		max-height: calc(100dvh - 60px - 1.5rem);
		overflow-y: auto;
		display: grid;
		padding: 1.125rem 1.25rem 1.25rem;
		border-radius: 1rem;
		background: hsl(var(--color-bg-primary));
		color: hsl(var(--color-text-primary));
		box-shadow:
			0 0 0 2px hsl(var(--color-green-100)),
			0 1px 2px hsl(var(--color-neutral-100) / 0.08),
			0 18px 40px -12px hsl(var(--color-neutral-100) / 0.35);
		animation: auf 0.22s cubic-bezier(0.16, 1, 0.3, 1);
	}
	@keyframes auf {
		from {
			opacity: 0;
			translate: 0 -0.5rem;
		}
	}
	.teil {
		display: grid;
		gap: 0.625rem;
	}
	.teil.darunter {
		margin-top: 1rem;
		padding-top: 1rem;
		border-top: 1px solid hsl(var(--color-border-primary));
	}
	h2 {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
		font-size: 1.125rem;
		line-height: 1.3;
		overflow-wrap: anywhere;
	}
	h2 :global(svg) {
		flex: none;
		width: 1.375rem;
		height: 1.375rem;
		margin-top: 0.05rem;
		color: hsl(var(--color-green-200));
	}
	h3 {
		font-size: 1rem;
	}
	p {
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
		text-wrap: pretty;
	}
	.klein {
		font-size: 0.875rem;
	}
	a {
		color: hsl(var(--color-text-primary));
		text-decoration: underline;
		text-underline-offset: 0.2em;
	}
	.knoepfe {
		display: flex;
		flex-wrap: wrap;
		gap: 0.625rem;
		margin-top: 0.125rem;
	}
	.fehler {
		margin-top: 0.75rem;
		color: hsl(var(--color-text-error));
		font-weight: 600;
	}
	@media (prefers-reduced-motion: reduce) {
		.zugriff {
			animation: none;
		}
	}
</style>

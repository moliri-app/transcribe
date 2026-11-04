<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Platzhalter für Sprache, die die Erkennung schon hört, aber noch nicht als Text hat: zufällige
     Kleinbuchstaben in Wörtern, die sich mehrmals in der Sekunde ändern, etwa 12 Zeichen je Sekunde
     Ton. Wer weniger Bewegung will, sieht stattdessen ein ruhig pulsierendes „…“. Nur fürs Auge. -->
<script>
	import { onMount } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';

	/** @type {{ g: import('./stand.svelte.js').Gehoert }} */
	let { g } = $props();

	const ZEICHEN_JE_SEKUNDE = 12;
	const HOECHSTENS = 140;
	const ABC = 'abcdefghijklmnopqrstuvwxyzäöüß';
	const ruhig = new MediaQuery('prefers-reduced-motion: reduce');

	let jetzt = $state(Date.now());
	onMount(() => {
		const uhr = setInterval(() => (jetzt = Date.now()), 160);
		return () => clearInterval(uhr);
	});

	const laenge = $derived(
		Math.min(HOECHSTENS, Math.round(ZEICHEN_JE_SEKUNDE * (g.pending + Math.max(0, jetzt - g.at) / 1000)))
	);

	// die Wortlängen bleiben stehen, damit die Zeile nicht springt; nur die Buchstaben wechseln
	/** @type {number[]} */
	const woerter = [];
	const buchstabe = () => ABC[Math.floor(Math.random() * ABC.length)];

	const text = $derived.by(() => {
		jetzt;
		let s = '';
		for (let i = 0; s.length < laenge; i++) {
			woerter[i] ??= 2 + Math.floor(Math.random() * 8);
			if (s) s += ' ';
			for (let k = 0; k < woerter[i]; k++) s += buchstabe();
		}
		return s.slice(0, laenge).trimEnd();
	});
</script>

{#if laenge > 0}
	{#if ruhig.current}
		<span class="platzhalter ruhig" aria-hidden="true">…</span>
	{:else}
		<span class="platzhalter" aria-hidden="true">{text}</span>
	{/if}
{/if}

<style>
	.platzhalter {
		font-style: italic;
		color: hsl(var(--color-text-secondary));
		opacity: 0.55;
		overflow-wrap: anywhere;
	}
	.ruhig {
		animation: pulsieren 1.6s ease-in-out infinite;
	}
	@keyframes pulsieren {
		50% {
			opacity: 0.2;
		}
	}
</style>

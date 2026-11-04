<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Knopf „Teilen“ mit Menü: den Text als neue E-Mail ins E-Mail-Programm übernehmen, oder über
     das Teilen des Systems an eine andere App geben (nur wo der Browser das kann). Ohne Betreff: den schreibt man selbst. -->
<script>
	import { tick } from 'svelte';
	import Button from '$lib/moliri/components/ui/Button.svelte';
	import Dropdown from '$lib/moliri/components/ui/Dropdown.svelte';
	import DropdownItem from '$lib/moliri/components/ui/DropdownItem.svelte';

	/** @type {{ text: string, beschreibung?: string, onmeldung: (m: string) => void }} */
	let { text, beschreibung, onmeldung } = $props();

	// Chrome gibt unter Windows längere Adressen gar nicht erst ans E-Mail-Programm weiter
	const MAILTO_MAX = 2000;

	const systemTeilen = typeof navigator !== 'undefined' && typeof navigator.share === 'function';

	let offen = $state(false);
	/** @type {HTMLElement | undefined} */
	let knopf = $state();
	/** @type {HTMLElement | undefined} */
	let menue = $state();

	/** die neue E-Mail mit dem Text, als Link; zu lang für den direkten Weg: keiner */
	const emailLink = $derived.by(() => {
		// Zeilenumbrüche in mailto sind CRLF (RFC 6068)
		const link = 'mailto:?body=' + encodeURIComponent(text.replace(/\r?\n/g, '\r\n'));
		return link.length <= MAILTO_MAX ? link : undefined;
	});

	/** Zu lang: der Text geht über die Zwischenablage, die E-Mail geht leer auf */
	async function emailLang() {
		try {
			await navigator.clipboard.writeText(text);
		} catch {
			onmeldung('Der Text ist zu lang für eine E-Mail per Klick, und Kopieren ging nicht.');
			return;
		}
		location.href = 'mailto:';
		onmeldung('Der Text ist zu lang, um ihn direkt mitzugeben. Er liegt in der Zwischenablage: in der E-Mail mit Strg+V einfügen.');
	}

	async function andereApp() {
		try {
			await navigator.share({ text });
			onmeldung('');
		} catch (e) {
			// Abbrechen im Teilen-Fenster ist kein Fehler
			if (/** @type {Error} */ (e).name !== 'AbortError') onmeldung('Teilen ging nicht.');
		}
	}

	const eintraege = () => /** @type {HTMLElement[]} */ ([...(menue?.querySelectorAll('[role="menuitem"]') ?? [])]);

	// Offen steht der Fokus auf dem ersten Eintrag (wie das Menü „Mehr“ der App-Leiste)
	$effect(() => {
		if (offen) tick().then(() => eintraege()[0]?.focus());
	});

	/** @param {KeyboardEvent} e */
	function taste(e) {
		const liste = eintraege();
		const i = liste.indexOf(/** @type {HTMLElement} */ (document.activeElement));
		const weiter = /** @type {Record<string, number>} */ ({ ArrowDown: i + 1, ArrowUp: i - 1, Home: 0, End: liste.length - 1 })[e.key];
		if (weiter !== undefined) {
			e.preventDefault();
			liste[(weiter + liste.length) % liste.length]?.focus();
		} else if (e.key === 'Escape') {
			offen = false;
			knopf?.querySelector('button')?.focus();
		}
	}

	/** @param {FocusEvent} e */
	function fokusWeg(e) {
		const nach = /** @type {Node | null} */ (e.relatedTarget);
		if (nach && !menue?.contains(nach) && !knopf?.contains(nach)) offen = false;
	}
</script>

<Dropdown bind:isOpen={offen} plainTrigger placement="bottom-end" minWidth="14rem">
	{#snippet trigger()}
		<span class="knopf" bind:this={knopf}>
			<Button type="tertiary" size="large" title={beschreibung} aria-haspopup="menu" aria-expanded={offen}>
				{#snippet iconLeft()}<IconMaterialSymbolsShareOutline />{/snippet}
				Teilen
			</Button>
		</span>
	{/snippet}
	{#snippet children()}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="eintraege" bind:this={menue} onkeydown={taste} onfocusout={fokusWeg}>
			<DropdownItem href={emailLink} onselected={emailLink ? () => onmeldung('') : emailLang} tabindex="-1">
				<IconMaterialSymbolsMailOutlineRounded />Als E-Mail
			</DropdownItem>
			{#if systemTeilen}
				<DropdownItem onselected={andereApp} tabindex="-1">
					<IconMaterialSymbolsIosShareRounded />An andere App …
				</DropdownItem>
			{/if}
		</div>
	{/snippet}
</Dropdown>

<style>
	.knopf {
		display: contents;
	}
	.eintraege {
		padding: 0.25rem 0;
	}
	.eintraege :global(.dropdown-item) {
		min-height: var(--hit);
		font-size: 1rem;
	}
</style>

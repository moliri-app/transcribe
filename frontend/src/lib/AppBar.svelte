<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- App-Leiste „Transcribe by Moliri“: Bibliothek, Editor, Live und Einstellungen, im
     Look von Moliris Hauptnavigation (Tintenleiste, aktiver Eintrag als Papier-Pille). Unter 560 px
     wandern die Einträge mit `more` in das Menü „Mehr“, damit nichts abgeschnitten wird. -->
<script>
	import { page } from '$app/state';
	import { onMount, tick } from 'svelte';
	import { auftraege } from '$lib/auftraege.svelte.js';
	import { einzeln } from '$lib/api.js';
	import { beobachteUpdate, update, versionText } from '$lib/update.svelte.js';
	import Dropdown from '$lib/moliri/components/ui/Dropdown.svelte';
	import DropdownItem from '$lib/moliri/components/ui/DropdownItem.svelte';
	import MoliriLogo from '$lib/MoliriLogo.svelte';
	import { MOLIRI } from '$lib/moliri-links.js';

	// Jeder Eintrag lädt die Seite neu vom Server: welche Oberfläche eine Seite hat, alte oder
	// neue, entscheidet internal/server/pages.go, nicht diese App.
	const tabs = [
		{ href: '/', label: 'Bibliothek', match: ['/', '/queue'], badge: true },
		{ href: '/editor', label: 'Editor', match: ['/editor'] },
		{ href: '/live', label: 'Live', match: ['/live'], more: true },
		{ href: '/settings', label: 'Einstellungen', match: ['/settings', '/einstellungen'], end: true, more: true }
	];
	const more = tabs.filter((t) => t.more);

	const path = $derived(page.url.pathname.replace(/\/+$/, '') || '/');
	/** die aktuelle Seite liegt im Menü „Mehr“: dann trägt dessen Knopf die Pille */
	const moreCurrent = $derived(more.some((t) => t.match.includes(path)));

	let menuOpen = $state(false);
	/** @type {HTMLButtonElement | undefined} */
	let moreButton = $state();
	/** @type {HTMLElement | undefined} */
	let menu = $state();

	const items = () => /** @type {HTMLElement[]} */ ([...(menu?.querySelectorAll('[role="menuitem"]') ?? [])]);

	// Offen steht der Fokus auf dem ersten Eintrag (Menü-Muster der ARIA-Praxis)
	$effect(() => {
		if (menuOpen) tick().then(() => items()[0]?.focus());
	});

	/** Pfeile, Pos1 und Ende wandern durch die Einträge; Esc schließt und gibt den Fokus zurück.
	 * @param {KeyboardEvent} e */
	function menuKey(e) {
		const list = items();
		const i = list.indexOf(/** @type {HTMLElement} */ (document.activeElement));
		const next = /** @type {Record<string, number>} */ ({ ArrowDown: i + 1, ArrowUp: i - 1, Home: 0, End: list.length - 1 })[e.key];
		if (next !== undefined) {
			e.preventDefault();
			list[(next + list.length) % list.length]?.focus();
		} else if (e.key === 'Escape') {
			menuOpen = false;
			moreButton?.focus();
		}
	}

	/** Mit Tab aus dem Menü heraus: es schließt sich.
	 * @param {FocusEvent} e */
	function menuFocusOut(e) {
		const to = /** @type {Node | null} */ (e.relatedTarget);
		if (to && !menu?.contains(to) && to !== moreButton) menuOpen = false;
	}

	/** Aufträge, die warten oder laufen */
	let active = $state(0);

	async function refresh() {
		try {
			const { jobs } = await (await fetch('/api/queue')).json();
			active = jobs.filter((/** @type {{status: string}} */ j) => ['queued', 'running', 'paused', 'canceling'].includes(j.status)).length;
		} catch {
			// Server gerade nicht erreichbar: Zahl bleibt stehen
		}
	}

	// sofort nach dem Einreihen oder Entfernen, nicht erst beim nächsten Abfragen
	$effect(() => {
		auftraege.stand;
		refresh();
	});

	onMount(() => {
		const t = setInterval(einzeln(refresh), 4000);
		const u = beobachteUpdate();
		return () => {
			clearInterval(t);
			u();
		};
	});

	/** eine neuere Version wartet auf den Klick, der sie installiert (Einstellungen, Über Transcribe) */
	const neu = $derived(['available', 'failed'].includes(update.stand?.state ?? '') ? update.stand?.latest : '');
</script>

<nav class="appbar" aria-label="Hauptnavigation">
	<!-- „Transcribe“ führt zur Bibliothek, „by Moliri“ auf Moliris Website (im Browser, neues Fenster) -->
	<div class="brand">
		<a class="home" href="/" data-sveltekit-reload aria-label="Transcribe by Moliri, Startseite">
			<svg viewBox="0 0 24 24" aria-hidden="true">
				<rect x="2.5" y="8" width="3.2" height="8" rx="1.6" fill="currentColor" />
				<rect x="7.9" y="3.5" width="3.2" height="17" rx="1.6" class="signal" />
				<rect x="13.3" y="6.5" width="3.2" height="11" rx="1.6" fill="currentColor" />
				<rect x="18.7" y="9.5" width="2.8" height="5" rx="1.4" fill="currentColor" />
			</svg>
			<span class="word">Transcribe</span>
		</a>
		<a class="by" href={MOLIRI} target="_blank" rel="noopener" title="Moliri, die Museumsdatenbank: Website öffnen">
			by <MoliriLogo class="mark" /> Moliri<span class="sr"> (öffnet die Website von Moliri)</span>
		</a>
	</div>
	<div class="tabs">
		{#each tabs as t (t.href)}
			{#if t.end && neu}
				<a class="tab update" href="/settings#programm" data-sveltekit-reload title="Version {versionText(neu)} ist da: in den Einstellungen installieren">
					<IconMaterialSymbolsUpgradeRounded aria-hidden="true" />Update
				</a>
			{/if}
			<a
				class="tab"
				class:end={t.end}
				class:more={t.more}
				href={t.href}
				data-sveltekit-reload
				aria-current={t.match.includes(path) ? 'page' : undefined}
			>
				{t.label}
				{#if t.badge && active > 0}
					<span class="badge" title="{active} aktive Aufträge">{active}<span class="sr"> aktive Aufträge</span></span>
				{/if}
			</a>
		{/each}
		<div class="more-menu">
			<Dropdown bind:isOpen={menuOpen} plainTrigger usePortal={false} placement="bottom-end" minWidth="15rem">
				{#snippet trigger()}
					<button
						bind:this={moreButton}
						type="button"
						class="tab"
						class:current={moreCurrent}
						aria-haspopup="menu"
						aria-expanded={menuOpen}
					>
						Mehr<span class="chevron" aria-hidden="true"><IconMaterialSymbolsKeyboardArrowDownRounded /></span>
					</button>
				{/snippet}
				{#snippet children()}
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div class="menu-items" bind:this={menu} onkeydown={menuKey} onfocusout={menuFocusOut}>
						{#each more as t (t.href)}
							<DropdownItem
								href={t.href}
								selected={t.match.includes(path)}
								aria-current={t.match.includes(path) ? 'page' : undefined}
								data-sveltekit-reload
								tabindex="-1">{t.label}</DropdownItem
							>
						{/each}
					</div>
				{/snippet}
			</Dropdown>
		</div>
	</div>
</nav>

<style>
	.appbar {
		position: sticky;
		top: 0;
		z-index: 60;
		display: flex;
		align-items: center;
		gap: 1rem;
		height: 60px;
		padding: 0 0.75rem 0 1.25rem;
		background: hsl(var(--color-bg-inverse));
		color: hsl(var(--color-text-inverse));
		font-size: 0.9375rem;
		font-weight: 600;
		line-height: 1;
	}

	.brand {
		display: flex;
		align-items: center;
		gap: 0.375rem;
		padding-right: 0.5rem;
		font-size: 1.0625rem;
		font-weight: 700;
		letter-spacing: -0.005em;
		white-space: nowrap;
		flex: none;
	}
	.home,
	.by {
		display: flex;
		align-items: center;
		min-height: var(--hit);
		text-decoration: none;
	}
	.home {
		gap: 0.625rem;
	}
	.home svg {
		width: 24px;
		height: 24px;
		flex: none;
	}
	.signal {
		fill: hsl(var(--color-green-100));
	}
	/* Nebelgrau auf Tinte: 14,6:1 */
	.by {
		gap: 0.3em;
		padding: 0 0.375rem;
		border-radius: 0.5rem;
		font-weight: 500;
		color: hsl(var(--color-neutral-10));
		transition: color 0.15s;
	}
	.by:hover {
		color: hsl(var(--color-text-inverse));
		text-decoration: underline;
		text-underline-offset: 0.2em;
	}
	/* Moliris Zeichen dreht sich beim Darüberfahren, wie im Kopf von www.moliri.app */
	.by :global(.mark) {
		width: 0.95em;
		height: 0.95em;
		transition: transform 500ms cubic-bezier(0.34, 1.56, 0.64, 1);
	}
	.by:hover :global(.mark) {
		transform: rotate(160deg);
	}
	@media (prefers-reduced-motion: reduce) {
		.by :global(.mark) {
			transition: none;
		}
	}

	.tabs {
		display: flex;
		align-items: center;
		gap: 0.25rem;
		flex: 1;
		min-width: 0;
	}

	.tab {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		min-height: var(--hit);
		padding: 0 1rem;
		border-radius: 2rem;
		color: hsl(var(--color-text-inverse));
		text-decoration: none;
		white-space: nowrap;
		cursor: pointer;
		transition: background-color 0.15s, color 0.15s;
	}
	.tab.end,
	.tab.update {
		margin-left: auto;
	}
	/* rechts steht das Update vor den Einstellungen (oder vor „Mehr“) */
	.tab.update ~ .tab.end,
	.tab.update ~ .more-menu {
		margin-left: 0;
	}
	/* Signalgrün wie die Zahl der Aufträge: hier wartet etwas auf einen Klick */
	.tab.update {
		gap: 0.375rem;
		background: hsl(var(--color-green-100));
		color: hsl(var(--color-neutral-100));
	}
	.tab.update:hover {
		background: color-mix(in oklab, hsl(var(--color-green-100)) 85%, hsl(var(--color-neutral-0)));
	}
	.tab.update :global(svg) {
		font-size: 1.25rem;
	}
	.tab:hover {
		background: color-mix(in oklab, hsl(var(--color-bg-inverse)) 82%, hsl(var(--color-text-inverse)));
	}
	.tab[aria-current='page'],
	.tab.current {
		background: hsl(var(--color-bg-primary));
		color: hsl(var(--color-text-primary));
	}
	.tab:focus-visible,
	.home:focus-visible,
	.by:focus-visible {
		outline: 2px solid hsl(var(--color-green-100));
		outline-offset: -2px;
		border-radius: 2rem;
	}

	/* Signalgrün mit Tinte: aktive Aufträge, wie „online“ in Moliri */
	.badge {
		min-width: 1.5rem;
		height: 1.5rem;
		padding: 0 0.4rem;
		border-radius: 999px;
		background: hsl(var(--color-green-100));
		color: hsl(var(--color-neutral-100));
		font-size: 0.8125rem;
		font-weight: 700;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		font-variant-numeric: tabular-nums;
	}

	.sr {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip: rect(0 0 0 0);
		white-space: nowrap;
	}

	/* „Mehr“ gibt es nur auf schmalen Bildschirmen */
	.more-menu {
		display: none;
		margin-left: auto;
	}
	button.tab {
		border: 0;
		background: none;
		font: inherit;
		gap: 0;
		padding-right: 0.5rem;
	}
	button.tab.current {
		background: hsl(var(--color-bg-primary));
	}
	.chevron {
		display: inline-flex;
		font-size: 1.25rem;
		transition: transform 0.15s;
	}
	.chevron :global(svg) {
		margin: 0;
	}
	button.tab[aria-expanded='true'] .chevron {
		transform: rotate(180deg);
	}
	.menu-items {
		display: flex;
		flex-direction: column;
		padding: 0.25rem 0;
	}
	.appbar .menu-items :global(.dropdown-item) {
		min-height: var(--hit);
		padding: 0 1.25rem;
		font-family: inherit;
		font-size: 1rem;
		font-weight: 500;
	}
	.appbar .menu-items :global(.dropdown-item:focus-visible) {
		outline: 2px solid hsl(var(--color-green-100));
		outline-offset: -2px;
	}

	@media (max-width: 760px) {
		.appbar {
			gap: 0.25rem;
			padding: 0 0.25rem 0 0.75rem;
		}
		.word,
		.by {
			display: none;
		}
		.tab {
			padding: 0 0.75rem;
		}
		button.tab {
			padding-right: 0.375rem;
		}
	}

	@media (max-width: 560px) {
		.tab.more {
			display: none;
		}
		.more-menu {
			display: flex;
		}
	}

	/* 360 px mit Zahl an „Bibliothek“ passt noch */
	@media (max-width: 400px) {
		.appbar {
			padding-left: 0.5rem;
		}
		.tab {
			padding: 0 0.625rem;
		}
		button.tab {
			padding-right: 0.25rem;
		}
	}
</style>

<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Die aus Moliri gespiegelten Komponenten (src/lib/moliri) an einem Ort, zum Ansehen und für die
     Port-Pakete als Nachschlagestelle. Dazu eine Schriftprobe: Atkinson Hyperlegible Next und
     Mono (wie in Moliri) in den Größen von Lesetext und Bedienung. -->
<script>
	import Button from '$lib/moliri/components/ui/Button.svelte';
	import Toggle from '$lib/moliri/components/ui/Toggle.svelte';
	import Tabs from '$lib/moliri/components/ui/Tabs.svelte';
	import Dropdown from '$lib/moliri/components/ui/Dropdown.svelte';
	import DropdownItem from '$lib/moliri/components/ui/DropdownItem.svelte';
	import Modal from '$lib/moliri/components/ui/Modal.svelte';
	import KeyboardTooltip from '$lib/moliri/components/ui/KeyboardTooltip.svelte';
	import LoadingWrapper from '$lib/moliri/components/ui/LoadingWrapper.svelte';

	let loading = $state(false);
	let mitlesen = $state(true);
	let sprecher = $state(false);
	let reiter = $state('lesen');
	let modal = $state(false);
	let qualitaet = $state('Ausgewogen');

	const probe =
		'Herr Berger erzählt, wie er 1958 als Lehrling in der Papierfabrik anfing: „Die Filze haben wir jeden Samstag gewaschen, von Hand.“';

	function laden() {
		loading = true;
		setTimeout(() => (loading = false), 1500);
	}
</script>

<svelte:head>
	<title>Bausteine · Transcribe by Moliri</title>
</svelte:head>

<main>
	<header>
		<h1>Bausteine</h1>
		<p>
			Die Komponenten, die Transcribe aus Moliri übernimmt. Sie liegen gespiegelt unter
			<code>frontend/src/lib/moliri</code> und werden nur in Moliri geändert.
		</p>
	</header>

	<section aria-labelledby="h-knoepfe">
		<h2 id="h-knoepfe">Knöpfe</h2>
		<div class="row">
			<Button type="primary">Speichern</Button>
			<Button type="secondary">Abbrechen</Button>
			<Button type="tertiary">Rückgängig</Button>
			<Button type="primary" variant="positive">Transkribieren</Button>
			<Button type="primary" variant="negative">Löschen</Button>
			<Button type="primary" size="large" {loading} onclick={laden}>
				{#snippet iconLeft()}<IconMaterialSymbolsPlayArrowRounded />{/snippet}
				Laden zeigen
			</Button>
		</div>
		<div class="row ink">
			<Button type="tertiary" variant="inverse">Auf Tinte</Button>
			<Button type="primary" variant="inverse">Papier auf Tinte</Button>
		</div>
	</section>

	<section aria-labelledby="h-schalter">
		<h2 id="h-schalter">Schalter</h2>
		<div class="stack">
			<Toggle bind:checked={mitlesen} title="Mitlesen" description="Markiert beim Abspielen das gesprochene Wort." />
			<Toggle
				bind:checked={sprecher}
				title="Sprecher überspringen"
				description="Ausgeblendete Sprecher werden beim Abspielen übersprungen."
				tooltip={{ title: 'Sprecher überspringen', content: 'Wirkt nur auf Sprecher, die im Editor ausgeblendet sind.' }}
			/>
		</div>
	</section>

	<section aria-labelledby="h-reiter">
		<h2 id="h-reiter">Reiter</h2>
		<Tabs
			bind:activeTab={reiter}
			tabs={[
				{ id: 'lesen', label: 'Lesen' },
				{ id: 'korrigieren', label: 'Korrigieren' },
				{ id: 'clips', label: 'Clips' }
			]}
		>
			{#snippet children(/** @type {string} */ id)}
				<p class="panel">Reiter „{id}“ ist offen.</p>
			{/snippet}
		</Tabs>
	</section>

	<section aria-labelledby="h-auswahl">
		<h2 id="h-auswahl">Auswahl, Dialog, Hinweis</h2>
		<div class="row">
			<Dropdown placement="bottom-start" minWidth="14rem">
				{#snippet trigger()}
					<span class="pick">Qualität: {qualitaet} <IconMaterialSymbolsKeyboardArrowDownRounded /></span>
				{/snippet}
				{#snippet children()}
					{#each ['Schnell', 'Ausgewogen', 'Genau'] as q (q)}
						<DropdownItem selected={q === qualitaet} onselected={() => (qualitaet = q)}>{q}</DropdownItem>
					{/each}
				{/snippet}
			</Dropdown>
			<Button type="secondary" onclick={() => (modal = true)}>Dialog öffnen</Button>
			<KeyboardTooltip textBefore="Abspielen" shortcuts={['Leertaste']}>
				<Button type="tertiary">
					{#snippet iconLeft()}<IconMaterialSymbolsKeyboardOutlineRounded />{/snippet}
					Mit Tastenkürzel
				</Button>
			</KeyboardTooltip>
		</div>
		<LoadingWrapper {loading} message="Lädt …">
			<p class="panel">Dieser Kasten zeigt „Lädt“, solange „Laden zeigen“ läuft.</p>
		</LoadingWrapper>
	</section>

	<section aria-labelledby="h-schrift">
		<h2 id="h-schrift">Schriftprobe</h2>
		<p>Lesetext im Editor (20px) und Bedienung (15px), links die Schrift der Oberfläche, rechts ihre Mono-Schwester für Code.</p>
		<div class="fonts">
			<figure style="font-family: var(--font-ui)">
				<figcaption>Atkinson Hyperlegible Next (--font-ui)</figcaption>
				<p class="lead">{probe}</p>
				<p class="ctl">Sprecher benennen · Clip anlegen · 00:12:48 · Il1 0O</p>
			</figure>
			<figure style="font-family: var(--font-mono)">
				<figcaption>Atkinson Hyperlegible Mono (--font-mono)</figcaption>
				<p class="lead">{probe}</p>
				<p class="ctl">Sprecher benennen · Clip anlegen · 00:12:48 · Il1 0O</p>
			</figure>
		</div>
	</section>
</main>

<Modal isOpen={modal} title="Sprecher zusammenlegen" size="small" onClose={() => (modal = false)}>
	<p>„Sprecher 2“ und „Herr Berger“ sind dieselbe Person. Zusammenlegen?</p>
	{#snippet actions()}
		<Button type="secondary" onclick={() => (modal = false)}>Abbrechen</Button>
		<Button type="primary" variant="positive" onclick={() => (modal = false)}>Zusammenlegen</Button>
	{/snippet}
</Modal>

<style>
	main {
		max-width: 60rem;
		margin: 0 auto;
		padding: 1.5rem 1.25rem 4rem;
	}
	header {
		margin-bottom: 1.25rem;
	}
	h1 {
		font-size: 1.75rem;
		margin-bottom: 0.5rem;
	}
	h2 {
		font-size: 0.85rem;
		font-weight: 600;
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: hsl(var(--color-text-secondary));
		margin-bottom: 0.75rem;
	}
	code {
		font-family: var(--font-mono);
		font-size: 0.9em;
	}
	section {
		background: hsl(var(--color-bg-primary));
		border-radius: 4px;
		padding: 1.25rem 1.5rem;
		margin-bottom: 1rem;
	}
	.row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 0.75rem;
	}
	.row.ink {
		background: hsl(var(--color-bg-inverse));
		border-radius: 8px;
		padding: 0.75rem;
	}
	.stack {
		display: grid;
		gap: 0.5rem;
		max-width: 32rem;
	}
	.panel {
		padding: 0.75rem 0;
	}
	.pick {
		display: inline-flex;
		align-items: center;
		min-height: var(--hit);
		padding: 0 0.75rem 0 1rem;
		border-radius: 6px;
		background: hsl(var(--color-bg-secondary));
		font-weight: 600;
		cursor: pointer;
	}
	.fonts {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1.5rem;
		margin-top: 0.75rem;
	}
	figcaption {
		font-family: var(--font-ui);
		font-size: 0.85rem;
		font-weight: 600;
		color: hsl(var(--color-text-secondary));
		margin-bottom: 0.5rem;
	}
	.lead {
		font-size: 20px;
		line-height: 1.72;
		margin-bottom: 0.5rem;
	}
	.ctl {
		font-size: 15px;
		font-weight: 600;
	}
	@media (max-width: 720px) {
		.fonts {
			grid-template-columns: 1fr;
		}
	}
</style>

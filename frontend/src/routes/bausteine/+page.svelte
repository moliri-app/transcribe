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
	const QUALITAETEN = [
{ id: 'schnell', name: /* @wc-include */ 'Fast' },
	{ id: 'ausgewogen', name: /* @wc-include */ 'Balanced' },
		{ id: 'genau', name: /* @wc-include */ 'Accurate' }
		];
		let qualitaet = $state('ausgewogen');
		
		const probe =
		/* @wc-include */ 'Mr Berger tells how he started as an apprentice at the paper mill in 1958: “We washed the felts by hand, every Saturday.”';

	function laden() {
		loading = true;
		setTimeout(() => (loading = false), 1500);
	}
</script>

<svelte:head>
	<title>Components · Transcribe by Moliri</title>
</svelte:head>

<main>
	<header>
		<h1>Components</h1>
		<p>
			The components Transcribe takes over from Moliri. They are mirrored under
			<code>frontend/src/lib/moliri</code> and are only changed in Moliri.
		</p>
	</header>

	<section aria-labelledby="h-knoepfe">
		<h2 id="h-knoepfe">Buttons</h2>
		<div class="row">
			<Button type="primary">Save</Button>
			<Button type="secondary">Cancel</Button>
			<Button type="tertiary">Undo</Button>
			<Button type="primary" variant="positive">Start transcription</Button>
			<Button type="primary" variant="negative">Delete</Button>
			<Button type="primary" size="large" {loading} onclick={laden}>
				{#snippet iconLeft()}<IconMaterialSymbolsPlayArrowRounded />{/snippet}
				Show loading
			</Button>
		</div>
		<div class="row ink">
			<Button type="tertiary" variant="inverse">On ink</Button>
			<Button type="primary" variant="inverse">Paper on ink</Button>
		</div>
	</section>

	<section aria-labelledby="h-schalter">
		<h2 id="h-schalter">Switches</h2>
		<div class="stack">
			<Toggle bind:checked={mitlesen} title="Follow along" description="Highlights the spoken word during playback." />
			<Toggle
				bind:checked={sprecher}
				title="Skip speakers"
				description="Hidden speakers are skipped during playback."
				tooltip={{ title: /* @wc-include */ 'Skip speakers', content: /* @wc-include */ 'Only affects speakers hidden in the editor.' }}
			/>
		</div>
	</section>

	<section aria-labelledby="h-reiter">
		<h2 id="h-reiter">Tabs</h2>
		<Tabs
			bind:activeTab={reiter}
			tabs={[
				{ id: 'lesen', label: /* @wc-include */ 'Read' },
				{ id: 'korrigieren', label: /* @wc-include */ 'Correct' },
				{ id: 'clips', label: /* @wc-include */ 'Clips' }
			]}
		>
			{#snippet children(/** @type {string} */ id)}
				<p class="panel">Tab “{id}” is open.</p>
			{/snippet}
		</Tabs>
	</section>

	<section aria-labelledby="h-auswahl">
		<h2 id="h-auswahl">Selection, dialog, hint</h2>
		<div class="row">
			<Dropdown placement="bottom-start" minWidth="14rem">
				{#snippet trigger()}
					<span class="pick">Quality: {QUALITAETEN.find((q) => q.id === qualitaet)?.name} <IconMaterialSymbolsKeyboardArrowDownRounded /></span>
				{/snippet}
				{#snippet children()}
					{#each QUALITAETEN as q (q.id)}
						<DropdownItem selected={q.id === qualitaet} onselected={() => (qualitaet = q.id)}>{q.name}</DropdownItem>
					{/each}
				{/snippet}
			</Dropdown>
			<Button type="secondary" onclick={() => (modal = true)}>Open dialog</Button>
			<KeyboardTooltip textBefore={/* @wc-include */ 'Play'} shortcuts={[/* @wc-include */ 'Spacebar']}>
				<Button type="tertiary">
					{#snippet iconLeft()}<IconMaterialSymbolsKeyboardOutlineRounded />{/snippet}
					With shortcut
				</Button>
			</KeyboardTooltip>
		</div>
		<LoadingWrapper {loading} message={/* @wc-include */ 'Loading …'}>
			<p class="panel">This box shows “Loading” while “Show loading” is running.</p>
		</LoadingWrapper>
	</section>

	<section aria-labelledby="h-schrift">
		<h2 id="h-schrift">Type sample</h2>
		<p>Body text in the editor (20px) and controls (15px): on the left the interface font, on the right its mono sister for code.</p>
		<div class="fonts">
			<figure style="font-family: var(--font-ui)">
				<figcaption>Atkinson Hyperlegible Next (--font-ui)</figcaption>
				<p class="lead">{probe}</p>
				<p class="ctl">Name speaker · Create clip · 00:12:48 · Il1 0O</p>
			</figure>
			<figure style="font-family: var(--font-mono)">
				<figcaption>Atkinson Hyperlegible Mono (--font-mono)</figcaption>
				<p class="lead">{probe}</p>
				<p class="ctl">Name speaker · Create clip · 00:12:48 · Il1 0O</p>
			</figure>
		</div>
	</section>
</main>

<Modal isOpen={modal} title="Merge speakers" size="small" onClose={() => (modal = false)}>
	<p>“Speaker 2” and “Mr Berger” are the same person. Merge?</p>
	{#snippet actions()}
		<Button type="secondary" onclick={() => (modal = false)}>Cancel</Button>
		<Button type="primary" variant="positive" onclick={() => (modal = false)}>Merge</Button>
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

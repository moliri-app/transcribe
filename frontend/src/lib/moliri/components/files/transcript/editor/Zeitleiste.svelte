<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/files/transcript/editor/Zeitleiste.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Ported from transcribe-go (frontend/src/lib/editor/Zeitleiste.svelte). -->
<!-- Die Zeitleiste: oben die Übersicht der ganzen Aufnahme mit dem sichtbaren Ausschnitt, darunter die
     Spuren Welle, Text, Sprecher, Sprache, Regie, Zweifel, Rotstift und Clips (ein Klick öffnet den Clip). Klicken springt, das Mausrad
     zoomt, Umschalt + Rad verschiebt; in der Übersicht verschiebt Ziehen den Ausschnitt. Ziehen in den
     Spuren markiert einen Bereich (Dialog wie bei einer Auswahl im Text), Ziehen an einem Zeitanker in
     der Spur „Text“ verschiebt seine Zeit. Über den Spuren zeigt ein Laufband den Text unter der Maus.
     Portiert aus web/editor.html. -->
<script>
	import * as m from '$lib/moliri/texte.js';
	import { untrack } from 'svelte';
	import { anchorBounds, cueName, fmt, langName, runs, wordLang } from './model.js';

	/** @type {{ ed: import('./editor.svelte.js').Editor }} */
	let { ed } = $props();

	/** @type {HTMLCanvasElement} */ let ov;
	/** @type {HTMLCanvasElement} */ let lanes;
	/** @type {HTMLElement} */ let root;
	let dpr = 1;
	let size = $state(0); // erhöht sich bei jeder Größenänderung, damit neu gezeichnet wird

	const LANES = /** @type {[string, number][]} */ ([
		['Welle', 70],
		['Text', 26],
		['Sprecher', 30],
		['Sprache', 24],
		['Regie', 22],
		['Zweifel', 12],
		['Rotstift', 18],
		['Clips', 22]
	]);
	/** Beschriftung einer Spur in der Sprache der Oberfläche (die Namen oben sind Schlüssel) */
	const laneLabel = (/** @type {string} */ n) =>
		({
			Welle: m.te_lane_wave,
			Text: m.te_lane_text,
			Sprecher: m.te_lane_speakers,
			Sprache: m.te_lane_language,
			Regie: m.te_lane_cues,
			Zweifel: m.te_lane_doubt,
			Rotstift: m.te_lane_corrections,
			Clips: m.te_clips
		})[n]?.() ?? n;
	const LABEL_W = 108;
	const RULER = 26; // Zeile des Zeitlineals, darin links die Zoomtasten

	// Spuren ohne jede Marke (keine Regie, kein Zweifel, keine Korrektur) entfallen: die Höhe gehört dem Text
	const active = $derived(
		LANES.filter(([n]) => {
			if (n === 'Regie') return (ed.final?.events || []).length > 0;
			if (n === 'Zweifel') return ed.words.some((w) => !w.edited && (w.fill || w.redone || (w.p !== undefined && w.p < 0.6)));
			if (n === 'Rotstift') return ed.ann.spans.length + ed.ann.edits.length > 0;
			if (n === 'Clips') return ed.ann.clips.length > 0;
			return true;
		})
	);
	// niedrige Bildschirme (Laptop 1366×768) und Handys: die Spuren rücken zusammen
	let scale = $state(1);
	$effect(() => {
		const low = matchMedia('(max-height: 820px), (max-width: 720px)'),
			mid = matchMedia('(max-height: 920px)');
		// im Fenster der Datei (nicht im Vollbild) ist wenig Höhe: die Spuren rücken immer zusammen
		const compact = ed.compact;
		const set = () => (scale = low.matches || compact ? 0.62 : mid.matches ? 0.85 : 1);
		set();
		low.addEventListener('change', set);
		mid.addEventListener('change', set);
		return () => (low.removeEventListener('change', set), mid.removeEventListener('change', set));
	});
	// jede Spur mindestens so hoch wie ihre Beschriftung
	const laneH = (/** @type {number} */ h) => Math.max(16, Math.round(h * scale));
	const laneGap = () => Math.round(6 * scale);
	const lanesH = $derived(RULER + active.reduce((t, [, h]) => t + laneH(h), 0) + laneGap() * (active.length - 1) + 4);

	/** Farben aus den Variablen der Seite, einmal je Zeichnen gelesen */
	/** @type {CSSStyleDeclaration} */ let cs;
	const css = (/** @type {string} */ n) => cs.getPropertyValue(n).trim();

	function resize() {
		dpr = devicePixelRatio || 1;
		for (const c of [ov, lanes]) {
			const r = c.getBoundingClientRect();
			c.width = Math.round(r.width * dpr);
			c.height = Math.round(r.height * dpr);
		}
		size++;
	}

	function laneY(/** @type {string} */ name) {
		let y = RULER;
		for (const [n, h] of active) {
			const hh = laneH(h);
			if (n === name) return [y, hh];
			y += hh + laneGap();
		}
		return [0, 0];
	}
	const x2t = (/** @type {number} */ x, /** @type {number} */ w) => ed.view.a + ((x - LABEL_W) / (w - LABEL_W)) * (ed.view.b - ed.view.a);
	const t2x = (/** @type {number} */ t, /** @type {number} */ w) => LABEL_W + ((t - ed.view.a) / (ed.view.b - ed.view.a)) * (w - LABEL_W);
	function roundRect(/** @type {CanvasRenderingContext2D} */ ctx, /** @type {number} */ x, /** @type {number} */ y, /** @type {number} */ w, /** @type {number} */ h, /** @type {number} */ r) {
		r = Math.max(0, Math.min(r, w / 2, h / 2));
		ctx.beginPath();
		ctx.roundRect(x, y, w, h, r);
	}

	// Balkenfeld: ein Balken je drei Pixel, der gespielte Teil in Tinte
	function drawWave(/** @type {CanvasRenderingContext2D} */ ctx, /** @type {number} */ x0, /** @type {number} */ w, /** @type {number} */ y, /** @type {number} */ h, /** @type {number} */ a, /** @type {number} */ b, /** @type {number | undefined} */ playT) {
		const pk = ed.peaks;
		if (!pk) return;
		const P = pk.peaks,
			step = pk.step,
			bar = 2,
			past = css('--wave-past'),
			fut = css('--wave');
		for (let x = 0; x < w; x += bar + 1) {
			const t0 = a + (x / w) * (b - a),
				t1 = a + ((x + bar) / w) * (b - a);
			let m = 0;
			for (let k = Math.floor(t0 / step); k <= Math.floor(t1 / step) && k < P.length; k++) if (P[k] > m) m = P[k];
			const v = Math.max(1, ((m / 255) * h) / 2);
			ctx.fillStyle = playT !== undefined && t0 < playT ? past : fut;
			ctx.fillRect(x0 + x, y + h / 2 - v, bar, 2 * v);
		}
	}

	const textLane = typeof document !== 'undefined' ? document.createElement('canvas') : /** @type {any} */ (null);

	function drawLanes() {
		if (!lanes?.width || !ed.final) return;
		cs = getComputedStyle(root);
		const ctx = /** @type {CanvasRenderingContext2D} */ (lanes.getContext('2d'));
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		const W = lanes.width / dpr,
			H = lanes.height / dpr,
			now = ed.time,
			view = ed.view,
			font = css('--font');
		ctx.clearRect(0, 0, W, H);
		ctx.textBaseline = 'middle';
		const span = view.b - view.a,
			stepT = [1, 2, 5, 10, 15, 30, 60, 120, 300, 600, 1200].find((s) => span / s < 12) || 1800;
		ctx.font = `500 13px ${font}`;
		ctx.strokeStyle = css('--rule');
		ctx.lineWidth = 1;
		for (let t = Math.ceil(view.a / stepT) * stepT; t < view.b; t += stepT) {
			const x = t2x(t, W);
			ctx.fillStyle = css('--ink-2');
			ctx.fillText(fmt(t), x + 4, RULER / 2 - 1);
			ctx.beginPath();
			ctx.moveTo(x + 0.5, RULER - 6);
			ctx.lineTo(x + 0.5, H);
			ctx.stroke();
		}
		ctx.font = `600 13px ${font}`;
		for (const [name] of active) {
			const [y, h] = laneY(name);
			ctx.fillStyle = css('--ink-2');
			ctx.fillText(laneLabel(name).toLocaleUpperCase(), 14, y + h / 2);
		}
		/** @type {(a: number, b: number, y: number, h: number, color: string, label?: string | null, opts?: { min?: number, alpha?: number, outline?: string | null, r?: number, ink?: string }) => void} */
		const bar = (a, b, y, h, color, label, opts = {}) => {
			if (b < view.a || a > view.b || h <= 0) return;
			const ca = Math.max(a, view.a),
				cb = Math.min(b, view.b),
				x0 = t2x(ca, W),
				x1 = t2x(cb, W),
				w = Math.max(opts.min ?? 2, x1 - x0 - 1);
			ctx.globalAlpha = opts.alpha ?? 1;
			ctx.fillStyle = color;
			roundRect(ctx, x0, y, w, h, opts.r ?? 4);
			ctx.fill();
			ctx.globalAlpha = 1;
			if (opts.outline) {
				ctx.strokeStyle = opts.outline;
				ctx.lineWidth = 1.5;
				roundRect(ctx, x0 + 0.75, y + 0.75, w - 1.5, h - 1.5, opts.r ?? 4);
				ctx.stroke();
			}
			if (label && w > 48) {
				ctx.save();
				ctx.beginPath();
				ctx.rect(x0, y, w, h);
				ctx.clip();
				ctx.fillStyle = opts.ink || '#ffffff';
				ctx.font = `600 12.5px ${font}`;
				ctx.fillText(label, x0 + 7, y + h / 2 + 0.5);
				ctx.restore();
			}
		};
		{
			const [y, h] = laneY('Welle');
			drawWave(ctx, LABEL_W, W - LABEL_W, y, h, view.a, view.b, now);
		}
		{
			const [y, h] = laneY('Text');
			drawTextLane(ctx, W, y, h, now);
		}
		{
			const [y, h] = laneY('Sprecher');
			for (const r of runs(ed.words, (w) => (w.speaker == null ? null : { k: w.speaker })))
				bar(r.start, r.end, y, h, css(`--spk${r.k % 8}`), ed.spkName(r.k), ed.hidden.has(r.k) ? { alpha: 0.2, ink: css('--ink-3') } : {});
		}
		{
			const [y, h] = laneY('Sprache');
			for (const r of runs(
				ed.words,
				(w) => {
					const L = wordLang(w, ed.ann);
					return { k: L.lang + '|' + L.by, lang: L.lang, by: L.by };
				},
				2
			)) {
				if (r.lang === 'de' && r.by === 'standard') bar(r.start, r.end, y, h, css('--desk'), langName('de'), { ink: css('--ink-2') });
				else
					bar(r.start, r.end, y, h, r.lang === 'de' ? css('--ink-3') : css(`--lang-${['en', 'fr', 'nl', 'it', 'es'].includes(r.lang) ? r.lang : 'x'}`), langName(r.lang) + (r.by === 'maschine' ? ` (${m.te_recognised()})` : ''), {
						outline: r.by === 'mensch' ? css('--red') : null
					});
			}
		}
		{
			const [y, h] = laneY('Regie');
			for (const e of ed.final.events || []) bar(e.start, e.end, y, h, css('--blue'), cueName(e.label), { alpha: e.label === 'Musik' || e.label === 'Gesang' ? 0.85 : 1 });
		}
		{
			const [y, h] = laneY('Zweifel');
			for (const w of ed.words) {
				if (w.edited || w.end < view.a || w.start > view.b) continue;
				let col = null,
					al = 1;
				if (w.fill) {
					bar(w.start, w.end, y, h, 'transparent', null, { outline: css('--fill'), r: 2, min: 3 });
					continue;
				}
				if (w.redone) col = css('--blue');
				else if (w.p !== undefined && w.p < 0.6) {
					col = css('--doubt');
					al = Math.min(1, 1.25 - w.p);
				}
				if (col) bar(w.start, w.end, y, h, col, null, { alpha: al, r: 2, min: 1.5 });
			}
		}
		{
			const [y, h] = laneY('Rotstift');
			for (const s of ed.ann.spans)
				bar(
					s.start,
					s.end,
					y,
					h,
					css('--red-soft'),
					[s.language && langName(s.language), s.speaker !== undefined && s.speaker !== '' && '→ ' + ed.spkName(s.speaker)].filter(Boolean).join(' · '),
					{ outline: css('--red'), ink: css('--red') }
				);
			for (const e of ed.ann.edits)
				if (e.mode === 'anchor') bar(e.end, e.end, y, h, css('--red'), null, { min: 2, r: 1 });
				else bar(e.start, e.end, y, h, css('--red'), null, { min: 3, r: 2 });
		}
		if (active.some(([n]) => n === 'Clips')) {
			const [y, h] = laneY('Clips');
			for (const c of ed.ann.clips)
				bar(c.start, c.end, y, h, css('--clip'), c.title, {
					ink: css('--clip-ink'),
					min: 3,
					outline: ed.playingClip === c.id ? css('--signal') : c.id === ed.activeClip ? css('--focus') : null
				});
		}
		// markierter Bereich: Auswahlfläche mit Kanten im Bedienakzent
		const sl = ed.sel;
		if (sl && sl.b > view.a && sl.a < view.b) {
			const x0 = t2x(Math.max(sl.a, view.a), W),
				x1 = t2x(Math.min(sl.b, view.b), W);
			ctx.fillStyle = css('--sel');
			ctx.fillRect(x0, RULER - 6, Math.max(2, x1 - x0), H - RULER + 6);
			ctx.fillStyle = css('--focus');
			if (sl.a >= view.a) ctx.fillRect(x0 - 1, RULER - 6, 2, H - RULER + 6);
			if (sl.b <= view.b) ctx.fillRect(x1 - 1, RULER - 6, 2, H - RULER + 6);
		}
		const hv = ed.hover;
		if (hv) {
			const x0 = t2x(Math.max(hv.a, view.a), W),
				x1 = t2x(Math.min(hv.b, view.b), W);
			ctx.save();
			ctx.fillStyle = css('--sel');
			ctx.fillRect(x0, RULER - 6, Math.max(2, x1 - x0), H - RULER + 6);
			ctx.strokeStyle = css('--focus');
			ctx.lineWidth = 2;
			ctx.strokeRect(x0, RULER - 5, Math.max(2, x1 - x0), H - RULER + 4);
			ctx.restore();
		}
		// Abspielkopf: Tintenlinie mit Signalgrün am Kopf
		const px = t2x(now, W);
		if (px >= LABEL_W && px <= W) {
			ctx.fillStyle = css('--ink');
			ctx.fillRect(px - 1, RULER - 6, 2, H - RULER + 6);
			ctx.beginPath();
			ctx.arc(px, RULER - 6, 5, 0, Math.PI * 2);
			ctx.fill();
			ctx.fillStyle = css('--signal');
			ctx.beginPath();
			ctx.arc(px, RULER - 6, 3, 0, Math.PI * 2);
			ctx.fill();
		}
	}

	// Spur „Text“: Zeitanker als kleine Fähnchen; nah herangezoomt steht der Text zwischen zwei Ankern
	// auf einer Zeile und blendet aus, wo er länger ist als der Platz bis zum nächsten Anker
	function drawTextLane(/** @type {CanvasRenderingContext2D} */ ctx, /** @type {number} */ W, /** @type {number} */ y, /** @type {number} */ h, /** @type {number} */ now) {
		const words = ed.words,
			anchors = ed.anchors,
			view = ed.view;
		if (!anchors.length) return;
		if (textLane.width !== Math.round(W * dpr) || textLane.height !== Math.round(h * dpr)) {
			textLane.width = Math.round(W * dpr);
			textLane.height = Math.round(h * dpr);
		}
		const c2 = /** @type {CanvasRenderingContext2D} */ (textLane.getContext('2d'));
		c2.setTransform(dpr, 0, 0, dpr, 0, 0);
		c2.clearRect(0, 0, W, h);
		c2.font = `15px ${css('--font')}`;
		c2.textBaseline = 'middle';
		const end = words[words.length - 1].end,
			ink = css('--ink'),
			ink2 = css('--future'),
			red = css('--red');
		for (let k = 0; k < anchors.length; k++) {
			const i0 = anchors[k],
				i1 = k + 1 < anchors.length ? anchors[k + 1] : words.length;
			const t0 = words[i0].start,
				t1 = i1 < words.length ? words[i1].start : end;
			if (t1 < view.a || t0 > view.b) continue;
			const x0 = t2x(t0, W),
				x1 = t2x(t1, W),
				left = Math.max(x0 + 5, LABEL_W),
				room = x1 - 4 - left;
			if (x1 - x0 < 26 || room < 12) continue;
			const seg = words.slice(i0, i1),
				txt = seg.map((w) => w.w).join(' ');
			c2.save();
			c2.beginPath();
			c2.rect(left, 0, room, h);
			c2.clip();
			let tx = x0 + 6;
			const sp = c2.measureText(' ').width;
			for (const w of seg) {
				if (tx > left + room) break;
				c2.fillStyle = w.edited ? red : w.start <= now ? ink : ink2;
				c2.fillText(w.w, tx, h / 2 + 1);
				tx += c2.measureText(w.w).width + sp;
			}
			c2.restore();
			if (x0 + 6 + c2.measureText(txt).width > left + room) {
				const fx = left + room,
					fw = Math.min(30, room);
				const g = c2.createLinearGradient(fx - fw, 0, fx, 0);
				g.addColorStop(0, 'rgba(0,0,0,0)');
				g.addColorStop(1, 'rgba(0,0,0,1)');
				c2.save();
				c2.globalCompositeOperation = 'destination-out';
				c2.fillStyle = g;
				c2.fillRect(fx - fw, 0, fw, h);
				c2.restore();
			}
		}
		ctx.drawImage(textLane, 0, 0, textLane.width, textLane.height, 0, y, W, h);
		const vis = visibleAnchors(W);
		const drag = ed.anchorDrag;
		for (const i of vis) {
			const on = drag?.i === i || hoverAnchor === i;
			const x = Math.round(t2x(drag?.i === i ? drag.t : words[i].start, W)) + 0.5;
			ctx.globalAlpha = on ? 1 : words[i].pin ? 0.85 : 0.45;
			ctx.fillStyle = on ? css('--blue') : words[i].pin ? red : css('--ink-3');
			ctx.fillRect(x - 0.75, y + 1, 1.5, h - 2);
			ctx.beginPath();
			ctx.moveTo(x - 0.75, y);
			ctx.lineTo(x + 6, y + 3.5);
			ctx.lineTo(x - 0.75, y + 7);
			ctx.closePath();
			ctx.fill();
			ctx.globalAlpha = 1;
		}
	}

	// Zeitanker, die in der Spur „Text“ als Fähnchen stehen (zu dicht: keine)
	function visibleAnchors(/** @type {number} */ W) {
		const words = ed.words,
			view = ed.view;
		const vis = ed.anchors.filter((i) => words[i].start >= view.a && words[i].start <= view.b);
		return vis.length > (W - LABEL_W) / 7 ? [] : vis;
	}
	function anchorAt(/** @type {number} */ x, /** @type {number} */ y, /** @type {number} */ W) {
		const [ly, lh] = laneY('Text');
		if (y < ly - 4 || y > ly + lh + 4) return -1;
		let best = -1,
			bd = 6;
		for (const i of visibleAnchors(W)) {
			const d = Math.abs(t2x(ed.words[i].start, W) - x);
			if (d < bd) ((bd = d), (best = i));
		}
		return best;
	}

	// Übersicht: Welle der ganzen Aufnahme, darunter ein Band der Sprecher; einmal gezeichnet, darüber
	// nur Ausschnitt und Abspielkopf
	/** @type {ImageData | null} */ let ovImg = null;
	function drawOverview() {
		if (!ov?.width || !ed.final) return;
		cs = getComputedStyle(root);
		const ctx = /** @type {CanvasRenderingContext2D} */ (ov.getContext('2d'));
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		const W = ov.width / dpr,
			H = ov.height / dpr,
			d = ed.duration(),
			x = (/** @type {number} */ t) => LABEL_W + (t / d) * (W - LABEL_W);
		ctx.clearRect(0, 0, W, H);
		ctx.fillStyle = css('--desk');
		ctx.fillRect(0, 0, W, H);
		let waveTop = 3;
		const th = ed.isVideo ? ed.thumbs : null;
		if (th) {
			// Filmstreifen: ein Vorschaubild je Kachel, auf die Kachelbreite beschnitten
			const m = th.meta,
				inner = W - LABEL_W,
				sh = H - 20,
				tw = Math.max(1, Math.round((sh * m.w) / m.h));
			const n = Math.max(1, Math.ceil(inner / tw)),
				cw = inner / n;
			for (let i = 0; i < n; i++) {
				const k = Math.max(0, Math.min(m.count - 1, Math.round((((i + 0.5) / n) * d) / m.step)));
				const sw = Math.min(m.w, (m.h * cw) / sh);
				ctx.drawImage(th.img, (k % m.cols) * m.w + (m.w - sw) / 2, Math.floor(k / m.cols) * m.h, sw, m.h, LABEL_W + i * cw, 1, cw + 0.5, sh);
			}
			ctx.fillStyle = 'rgba(0,0,0,.28)';
			ctx.fillRect(LABEL_W, 1, inner, sh);
			waveTop = sh - 6;
		}
		drawWave(ctx, LABEL_W, W - LABEL_W, waveTop, H - 12 - (waveTop - 3), 0, d, undefined);
		for (const r of runs(ed.words, (w) => (w.speaker == null ? null : { k: w.speaker }), 3)) {
			ctx.globalAlpha = ed.hidden.has(r.k) ? 0.2 : 1;
			ctx.fillStyle = css(`--spk${r.k % 8}`);
			ctx.fillRect(x(r.start), H - 6, Math.max(1, x(r.end) - x(r.start)), 4);
		}
		ctx.globalAlpha = 1;
		ctx.fillStyle = css('--clip');
		for (const c of ed.ann.clips) ctx.fillRect(x(c.start), H - 11, Math.max(2, x(c.end) - x(c.start)), 4);
		for (const e of ed.ann.edits) {
			ctx.fillStyle = css('--red');
			ctx.fillRect(x(e.start) - 1, 0, 2, 5);
		}
		ctx.fillStyle = css('--ink-2');
		ctx.font = `600 13px ${css('--font')}`;
		ctx.textBaseline = 'middle';
		ctx.fillText(m.te_overview().toLocaleUpperCase(), 14, H / 2);
		ovImg = ctx.getImageData(0, 0, ov.width, ov.height);
	}
	function drawOverviewView() {
		if (!ov?.width || !ovImg) return;
		const ctx = /** @type {CanvasRenderingContext2D} */ (ov.getContext('2d'));
		ctx.putImageData(ovImg, 0, 0);
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		const W = ov.width / dpr,
			H = ov.height / dpr,
			d = ed.duration(),
			x = (/** @type {number} */ t) => LABEL_W + (t / d) * (W - LABEL_W);
		const v = ed.view;
		ctx.fillStyle = css('--sel');
		if (v.b - v.a < d * 0.98) ctx.fillRect(x(v.a), 0, Math.max(3, x(v.b) - x(v.a)), H);
		ctx.strokeStyle = css('--focus');
		ctx.lineWidth = 2;
		ctx.strokeRect(x(v.a), 1, Math.max(3, x(v.b) - x(v.a)), H - 2);
		ctx.fillStyle = css('--ink');
		ctx.fillRect(x(ed.time) - 1, 0, 2, H);
	}

	$effect(() => {
		size;
		ed.final;
		ed.peaks;
		ed.words;
		ed.hidden;
		ed.ann;
		ed.thumbs;
		ed.mediaDuration;
		drawOverview();
	});
	$effect(() => {
		size;
		ed.time;
		ed.view;
		ed.hover;
		ed.sel;
		ed.anchorDrag;
		hoverAnchor;
		ed.words;
		ed.hidden;
		ed.ann;
		ed.activeClip;
		ed.playingClip;
		drawLanes();
		drawOverviewView();
	});
	$effect(() => {
		untrack(resize);
		const ro = new ResizeObserver(resize);
		ro.observe(lanes);
		document.fonts?.ready.then(resize);
		return () => ro.disconnect();
	});

	// ---- Maus ----
	let tip = $state(/** @type {{ x: number, y: number, head: string, words: { w: string, c?: string, red: boolean }[], k: number, off: number } | null} */ (null));
	/** @type {HTMLElement | undefined} */ let strip = $state();
	/** @type {HTMLElement | undefined} */ let win = $state();

	// Klicken springt, Ziehen markiert einen Bereich, Ziehen an einem Anker verschiebt seine Zeit
	let hoverAnchor = $state(-1);
	/** @type {{ x: number, y: number, t: number } | null} */ let down = null;
	/** @type {{ i: number, t0: number, lo: number, hi: number } | null} */ let adrag = null;
	function lanesDown(/** @type {MouseEvent} */ e) {
		const r = lanes.getBoundingClientRect();
		if (e.button !== 0 || e.clientX - r.left < LABEL_W) return;
		e.preventDefault();
		ed.range = null;
		const ai = ed.canEdit ? anchorAt(e.clientX - r.left, e.clientY - r.top, r.width) : -1;
		if (ai >= 0) {
			ed.textEdit?.commit();
			const words = ed.words,
				bd = anchorBounds(words, ed.anchors, ai);
			adrag = { i: ai, t0: words[ai].start, lo: bd.lo, hi: bd.hi };
			ed.anchorDrag = { i: ai, t: adrag.t0 };
			tip = null;
			return;
		}
		down = { x: e.clientX, y: e.clientY, t: x2t(e.clientX - r.left, r.width) };
		ed.sel = null;
	}
	function windowMove(/** @type {MouseEvent} */ e) {
		if (ovDrag) return ovCenter(e);
		if (!adrag && !down) return;
		const r = lanes.getBoundingClientRect(),
			t = x2t(e.clientX - r.left, r.width);
		if (adrag) ed.anchorDrag = { i: adrag.i, t: Math.max(adrag.lo, Math.min(adrag.hi, t)) };
		else if (down && Math.abs(e.clientX - down.x) >= 4) ed.sel = { a: Math.max(0, Math.min(down.t, t)), b: Math.min(ed.duration(), Math.max(down.t, t)) };
	}
	function windowUp(/** @type {MouseEvent} */ e) {
		ovDrag = false;
		if (adrag) {
			const a = adrag,
				t = ed.anchorDrag?.t ?? a.t0;
			adrag = null;
			ed.anchorDrag = null;
			if (Math.abs(t - a.t0) > 0.01) ed.moveAnchorTo(a.t0, t);
			return;
		}
		if (!down) return;
		const d = down;
		down = null;
		if (Math.abs(e.clientX - d.x) < 4 || !ed.sel) {
			// ein Klick auf einen Clip in seiner Spur öffnet ihn
			const r = lanes.getBoundingClientRect(),
				onClips = active.some(([n]) => n === 'Clips') && (() => {
					const [cy, ch] = laneY('Clips');
					return d.y - r.top >= cy && d.y - r.top <= cy + ch;
				})(),
				hit = onClips && ed.ann.clips.find((c) => c.start <= d.t && d.t <= c.end);
			if (hit) {
				ed.showClip(hit);
				ed.clipDlg = { kind: 'clip', clip: hit, x: e.clientX, y: e.clientY };
				return;
			}
			return ed.seek(d.t);
		}
		ed.range = { ...ed.sel, x: e.clientX, y: e.clientY };
	}
	function lanesMove(/** @type {MouseEvent} */ e) {
		const r = lanes.getBoundingClientRect();
		if (!adrag) hoverAnchor = e.clientX - r.left < LABEL_W ? -1 : anchorAt(e.clientX - r.left, e.clientY - r.top, r.width);
		if (adrag || down) return (tip = null);
		if (e.clientX - r.left < LABEL_W || !ed.words.length) return (tip = null);
		const t = x2t(e.clientX - r.left, r.width),
			words = ed.words;
		let i0 = words.findIndex((w) => w.end > t - 12);
		if (i0 < 0) i0 = words.length - 1;
		let i1 = i0;
		while (i1 < words.length && words[i1].start < t + 12) i1++;
		const seg = words.slice(i0, Math.max(i0 + 1, i1));
		const w0 = words.find((w) => w.start <= t && w.end >= t - 1),
			ev = (ed.final.events || []).find((/** @type {any} */ v) => v.start <= t && v.end >= t);
		const L = w0 ? wordLang(w0, ed.ann) : null;
		let k = seg.findIndex((w) => w.end >= t);
		if (k < 0) k = seg.length - 1;
		const w = seg[k],
			off = t >= w.start ? Math.min(1, (t - w.start) / Math.max(0.05, w.end - w.start)) : 0;
		tip = {
			x: Math.max(8, Math.min(e.clientX - 220, innerWidth - 448)),
			y: e.clientY + 20,
			head: `${fmt(t)}${w0?.speaker != null ? ' · ' + ed.spkName(w0.speaker) : ''}${L && L.lang !== 'de' ? ' · ' + langName(L.lang) : ''}${ev ? ' · ' + cueName(ev.label) : ''}`,
			words: seg.map((w) => ({ w: w.fill ? `(${w.w})` : w.w, c: w.speaker != null ? `var(--spk${w.speaker % 8})` : undefined, red: !!w.edited })),
			k: t >= w.start ? k : -1,
			off: t >= w.start ? off : 0
		};
	}
	// das Laufband gleitet so, dass die Stelle unter der Maus in der Mitte steht
	$effect(() => {
		if (!tip || !strip || !win) return;
		const el = /** @type {HTMLElement | undefined} */ (strip.children[Math.max(0, tip.k)]);
		const x = el ? el.offsetLeft + el.offsetWidth * tip.off : 0;
		strip.style.transform = `translateX(${Math.round(win.clientWidth / 2 - x)}px)`;
	});

	function wheel(/** @type {WheelEvent} */ e) {
		e.preventDefault();
		const r = lanes.getBoundingClientRect(),
			v = ed.view,
			w = v.b - v.a;
		if (e.shiftKey || Math.abs(e.deltaX) > Math.abs(e.deltaY)) {
			const dt = ((e.deltaX || e.deltaY) / r.width) * w;
			return ed.setView(v.a + dt, v.b + dt);
		}
		const t = x2t(Math.max(LABEL_W, e.clientX - r.left), r.width),
			f = e.deltaY > 0 ? 1.25 : 0.8;
		ed.setView(t - (t - v.a) * f, t + (v.b - t) * f);
	}
	$effect(() => {
		lanes.addEventListener('wheel', wheel, { passive: false });
		return () => lanes.removeEventListener('wheel', wheel);
	});

	let ovDrag = false;
	const ovT = (/** @type {MouseEvent} */ e) => {
		const r = ov.getBoundingClientRect();
		return Math.max(0, Math.min(ed.duration(), ((e.clientX - r.left - LABEL_W) / (r.width - LABEL_W)) * ed.duration()));
	};
	function ovCenter(/** @type {MouseEvent} */ e) {
		const t = ovT(e),
			w = ed.view.b - ed.view.a;
		ed.setView(t - w / 2, t + w / 2);
	}
</script>

<svelte:window onmousemove={windowMove} onmouseup={windowUp} />

<div class="zeitleiste" bind:this={root}>
	<canvas
		class="ov"
		class:video={ed.isVideo && ed.thumbs}
		bind:this={ov}
		aria-label={m.te_overview_hint()}
		onmousedown={(e) => {
			if (e.clientX - ov.getBoundingClientRect().left < LABEL_W) return;
			ovDrag = true;
			ovCenter(e);
		}}
		ondblclick={(e) => {
			if (e.clientX - ov.getBoundingClientRect().left >= LABEL_W) ed.seek(ovT(e));
		}}
	></canvas>
	<div class="lanes-wrap">
		<canvas
			class="lanes"
			bind:this={lanes}
			style:height="{lanesH}px"
			class:anker={hoverAnchor >= 0}
			aria-label={m.te_lanes_hint()}
			onmousedown={lanesDown}
			onmousemove={lanesMove}
			onmouseleave={() => {
				tip = null;
				hoverAnchor = -1;
			}}
		></canvas>
		<div class="zoom" role="group" aria-label={m.te_zoom()}>
			<button title="{m.te_zoom_out()} (−)" aria-label={m.te_zoom_out()} onclick={() => ed.zoom(2)}><IconMaterialSymbolsRemoveRounded /></button>
			<button title="{m.te_zoom_in()} (+)" aria-label={m.te_zoom_in()} onclick={() => ed.zoom(0.5)}><IconMaterialSymbolsAddRounded /></button>
			<button title={m.te_zoom_all()} aria-label={m.te_zoom_all()} onclick={() => ed.setView(0, ed.duration())}><IconMaterialSymbolsFitScreenOutlineRounded /></button>
		</div>
	</div>
</div>

{#if tip}
	<div class="tip" style:left="{tip.x}px" style:top="{tip.y}px" aria-hidden="true">
		<div class="head">{tip.head}</div>
		<div class="win" bind:this={win}>
			<div class="strip" bind:this={strip}>
				{#each tip.words as w, k (k)}<span class:on={k === tip.k} class:red={w.red} style:border-bottom-color={w.c}>{w.w}</span>{/each}
			</div>
			<div class="mid"></div>
		</div>
	</div>
{/if}

<style>
	.zeitleiste {
		background: var(--paper);
		border-bottom: 1px solid var(--rule);
		position: relative;
		user-select: none;
		box-shadow: 0 6px 16px -12px hsl(var(--color-neutral-100) / 0.2);
		z-index: 2;
	}
	canvas {
		display: block;
		width: 100%;
	}
	.ov {
		height: 34px;
		cursor: pointer;
	}
	.ov.video {
		height: 58px;
	}
	.lanes-wrap {
		position: relative;
	}
	.lanes {
		cursor: pointer;
	}
	.lanes.anker {
		cursor: ew-resize;
	}
	.zoom {
		position: absolute;
		left: 4px;
		top: 0;
		display: flex;
		gap: 2px;
	}
	.zoom button {
		width: 32px;
		height: 26px;
		border: 0;
		border-radius: 1rem;
		background: transparent;
		color: var(--ink-2);
		display: grid;
		place-items: center;
		padding: 0;
		cursor: pointer;
	}
	.zoom button:hover {
		background: var(--desk);
		color: var(--ink);
	}
	.zoom :global(svg) {
		width: 20px;
		height: 20px;
		margin: 0;
		padding: 0;
	}

	.tip {
		position: fixed;
		pointer-events: none;
		background: var(--head);
		color: var(--head-ink);
		font-size: 0.8125rem;
		border-radius: 0.875rem;
		width: 440px;
		z-index: 50;
		padding: 0.5rem 0 0.625rem;
		box-shadow: 0 10px 28px -8px hsl(var(--color-neutral-100) / 0.45);
	}
	.tip .head {
		padding: 0 0.875rem 0.25rem;
		color: var(--head-ink-2);
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.win {
		position: relative;
		height: 26px;
		overflow: hidden;
		font-size: 1rem;
		mask-image: linear-gradient(90deg, transparent, #000 14%, #000 86%, transparent);
	}
	.strip {
		position: absolute;
		left: 0;
		top: 0;
		white-space: nowrap;
		will-change: transform;
	}
	.strip span {
		padding: 0 3px;
		border-bottom: 2px solid transparent;
	}
	.strip span.red {
		color: var(--red-on-ink);
	}
	.strip span.on {
		color: var(--signal);
	}
	.mid {
		position: absolute;
		left: 50%;
		top: 2px;
		bottom: 2px;
		width: 1.5px;
		background: var(--signal);
		opacity: 0.85;
	}
</style>

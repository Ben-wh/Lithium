<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { page } from '$app/state';
	import { adminSettings } from '$lib/api';
	import { backgrounds, type Background } from '$lib/backgrounds';
	import Icon from '$lib/components/Icon.svelte';
	import Toggle from '$lib/components/Toggle.svelte';
	import { accents, applyAccent, type Accent } from '$lib/accents';
	import BrandingSettings from './BrandingSettings.svelte';

	// Pick the background everyone sees. Saves as soon as one is chosen.
	let current = $derived<Background>(page.data.settings?.background ?? 'none');
	let saving = $state(false);
	let status = $state<'saved' | 'error' | null>(null);
	let statusTimer: ReturnType<typeof setTimeout>;

	// --- Accent colour: previews instantly, saves as soon as one is chosen ---
	let accent = $derived<Accent>(page.data.settings?.accent ?? 'coral');
	let accentSaving = $state(false);
	let accentStatus = $state<'saved' | 'error' | null>(null);
	let accentTimer: ReturnType<typeof setTimeout>;

	async function chooseAccent(next: Accent) {
		if (next === accent || accentSaving) return;
		const previous = accent;
		accent = next;
		applyAccent(next);
		accentSaving = true;
		clearTimeout(accentTimer);
		try {
			await adminSettings.update({ accent: next });
			await invalidate('app:settings');
			accentStatus = 'saved';
		} catch {
			accent = previous;
			applyAccent(previous);
			accentStatus = 'error';
		} finally {
			accentSaving = false;
			accentTimer = setTimeout(() => (accentStatus = null), accentStatus === 'saved' ? 2000 : 5000);
		}
	}

	// --- Sensitive mode ---
	const DEFAULT_MESSAGE = 'Sensitive system: handle all data in line with policy';
	let sensitive = $derived(page.data.settings?.sensitiveMode ?? false);
	let message = $state(page.data.settings?.sensitiveMessage ?? DEFAULT_MESSAGE);
	const savedMessage = $derived(page.data.settings?.sensitiveMessage ?? DEFAULT_MESSAGE);
	const messageChanged = $derived(message.trim() !== savedMessage);
	const preview = $derived(message.trim() || DEFAULT_MESSAGE);

	// Its own saving state, so its feedback shows on this card rather than the
	// Background card's, and saving it doesn't lock the background picker.
	let sensitiveSaving = $state(false);
	let sensitiveStatus = $state<'saved' | 'error' | null>(null);
	let sensitiveTimer: ReturnType<typeof setTimeout>;

	async function saveSensitive(patch: { sensitiveMode?: boolean; sensitiveMessage?: string }, revert: () => void) {
		sensitiveSaving = true;
		clearTimeout(sensitiveTimer);
		try {
			await adminSettings.update(patch);
			await invalidate('app:settings'); // the banner appears or changes straight away
			message = page.data.settings?.sensitiveMessage ?? message;
			sensitiveStatus = 'saved';
		} catch {
			revert();
			sensitiveStatus = 'error';
		} finally {
			sensitiveSaving = false;
			sensitiveTimer = setTimeout(() => (sensitiveStatus = null), sensitiveStatus === 'saved' ? 2000 : 5000);
		}
	}

	async function choose(bg: Background) {
		if (bg === current || saving) return;
		const previous = current;
		current = bg;
		saving = true;
		clearTimeout(statusTimer);
		try {
			await adminSettings.update({ background: bg });
			await invalidate('app:settings'); // applies straight away, here and everywhere else
			status = 'saved';
		} catch {
			current = previous;
			status = 'error';
		} finally {
			saving = false;
			statusTimer = setTimeout(() => (status = null), status === 'saved' ? 2000 : 5000);
		}
	}
</script>

<BrandingSettings />

<div class="mt-5 rounded-xl border border-ink-800 bg-ink-900/60">
	<div class="flex items-center justify-between gap-3 border-b border-ink-800 px-5 py-4">
		<div>
			<h3 class="text-sm font-semibold">Accent colour</h3>
			<p class="mt-0.5 text-sm text-ink-400">Used for buttons, links and highlights. Errors and urgent tickets always stay red.</p>
		</div>
		<span class="shrink-0 text-xs" aria-live="polite">
			{#if accentSaving}
				<span class="text-ink-500">Saving…</span>
			{:else if accentStatus === 'saved'}
				<span class="flex items-center gap-1 text-ink-400"><Icon name="check" class="size-3.5 text-primary-300" /> Saved</span>
			{:else if accentStatus === 'error'}
				<span class="text-danger-300">Couldn't save. Try again.</span>
			{/if}
		</span>
	</div>
	<div role="radiogroup" aria-label="Accent colour" class="flex flex-wrap gap-2 p-5">
		{#each accents as a (a.value)}
			{@const selected = accent === a.value}
			<button
				type="button"
				role="radio"
				aria-checked={selected}
				onclick={() => chooseAccent(a.value)}
				class={[
					'flex items-center gap-2 rounded-full border py-1.5 pr-3.5 pl-1.5 text-sm transition focus-visible:ring-2 focus-visible:ring-primary-400/50 focus-visible:outline-none',
					selected ? 'border-ink-500 bg-ink-850 font-medium text-ink-100' : 'border-ink-700 text-ink-400 hover:border-ink-500 hover:text-ink-100'
				]}
			>
				<span class="flex size-6 items-center justify-center rounded-full" style:background-color={a.swatch}>
					{#if selected}<Icon name="check" class="size-3.5 text-black/70" />{/if}
				</span>
				{a.label}
			</button>
		{/each}
	</div>
</div>

<div class="mt-5 rounded-xl border border-ink-800 bg-ink-900/60">
	<div class="flex items-center justify-between gap-3 border-b border-ink-800 px-5 py-4">
		<h3 class="text-sm font-semibold">Sensitive mode</h3>
		<span class="shrink-0 text-xs" aria-live="polite">
			{#if sensitiveSaving}
				<span class="text-ink-500">Saving…</span>
			{:else if sensitiveStatus === 'saved'}
				<span class="flex items-center gap-1 text-ink-400"><Icon name="check" class="size-3.5 text-primary-300" /> Saved</span>
			{:else if sensitiveStatus === 'error'}
				<span class="text-danger-300">Couldn't save. Try again.</span>
			{/if}
		</span>
	</div>
	<div class="space-y-4 px-5 py-4">
		<Toggle
			bind:checked={sensitive}
			disabled={sensitiveSaving}
			onchange={(on) => saveSensitive({ sensitiveMode: on }, () => (sensitive = !on))}
		>
			Show a sensitive mode banner
			{#snippet description()}
				A red bar with your message stays above the navbar on every page, for everyone signed in.
			{/snippet}
		</Toggle>
		<form
			onsubmit={(e) => {
				e.preventDefault();
				saveSensitive({ sensitiveMessage: message }, () => {});
			}}
		>
			<label for="sensitive-message" class="mb-1.5 block text-[13px] font-medium text-ink-200">Banner message</label>
			<div class="flex flex-wrap gap-2">
				<input
					id="sensitive-message"
					bind:value={message}
					maxlength="120"
					placeholder={DEFAULT_MESSAGE}
					class="field h-9 min-w-0 flex-1 text-[13px]"
				/>
				<button
					type="submit"
					disabled={sensitiveSaving || !messageChanged}
					class="h-9 rounded-lg bg-primary-400 px-3 text-[13px] font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-40"
				>
					Save
				</button>
			</div>
			<p class="mt-1.5 text-xs text-ink-500">Up to 120 characters. Leave blank for the default message.</p>
			<p class="mt-3 mb-1.5 text-xs text-ink-500">Preview</p>
			<div class="flex items-center justify-center gap-2 rounded-md bg-[#c53030] px-4 py-1.5 text-center text-[13px] font-semibold text-white">
				<Icon name="shieldAlert" class="size-4 shrink-0" />
				<span class="min-w-0 break-words">{preview}</span>
			</div>
		</form>
	</div>
</div>

<div class="mt-5 rounded-xl border border-ink-800 bg-ink-900/60">
	<div class="flex items-center justify-between gap-3 border-b border-ink-800 px-5 py-4">
		<div>
			<h3 class="text-sm font-semibold">Background</h3>
			<p class="mt-0.5 text-sm text-ink-400">Shown behind every page for everyone signed in. Works in light and dark themes.</p>
		</div>
		<span class="shrink-0 text-xs" aria-live="polite">
			{#if saving}
				<span class="text-ink-500">Saving…</span>
			{:else if status === 'saved'}
				<span class="flex items-center gap-1 text-ink-400"><Icon name="check" class="size-3.5 text-primary-300" /> Saved</span>
			{:else if status === 'error'}
				<span class="text-danger-300">Couldn't save. Try again.</span>
			{/if}
		</span>
	</div>
	<div role="radiogroup" aria-label="Background" class="grid grid-cols-2 gap-3 p-5 sm:grid-cols-3">
		{#each backgrounds as bg (bg.value)}
			{@const selected = current === bg.value}
			<button
				type="button"
				role="radio"
				aria-checked={selected}
				onclick={() => choose(bg.value)}
				class={[
					'group overflow-hidden rounded-xl border text-left transition focus-visible:ring-2 focus-visible:ring-primary-400/50 focus-visible:outline-none',
					selected ? 'border-primary-400 ring-1 ring-primary-400' : 'border-ink-700 hover:border-ink-500'
				]}
			>
				<!-- Preview: the pattern behind a miniature card. -->
				<div class={['relative h-24 bg-ink-950', `app-bg-${bg.value}`]}>
					<div class="absolute inset-x-5 top-5 h-9 rounded-md border border-ink-800 bg-ink-900/80"></div>
					<div class="absolute inset-x-5 top-16 h-2 w-1/2 rounded-full bg-ink-800"></div>
				</div>
				<div class="flex items-center justify-between border-t border-ink-800 px-3 py-2">
					<span class={['text-sm', selected ? 'font-medium text-ink-100' : 'text-ink-400 group-hover:text-ink-100']}>{bg.label}</span>
					{#if selected}<Icon name="check" class="size-4 text-primary-400" />{/if}
				</div>
			</button>
		{/each}
	</div>
</div>

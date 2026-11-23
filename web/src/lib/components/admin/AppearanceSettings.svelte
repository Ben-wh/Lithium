<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { page } from '$app/state';
	import { adminSettings } from '$lib/api';
	import { backgrounds, type Background } from '$lib/backgrounds';
	import Icon from '$lib/components/Icon.svelte';
	import Toggle from '$lib/components/Toggle.svelte';

	// Pick the background everyone sees. Saves as soon as one is chosen.
	let current = $derived<Background>(page.data.settings?.background ?? 'none');
	let saving = $state(false);
	let status = $state<'saved' | 'error' | null>(null);
	let statusTimer: ReturnType<typeof setTimeout>;

	// --- Sensitive mode ---
	const DEFAULT_MESSAGE = 'Sensitive system: handle all data in line with policy';
	let sensitive = $derived(page.data.settings?.sensitiveMode ?? false);
	let message = $state(page.data.settings?.sensitiveMessage ?? DEFAULT_MESSAGE);
	const savedMessage = $derived(page.data.settings?.sensitiveMessage ?? DEFAULT_MESSAGE);
	const messageChanged = $derived(message.trim() !== savedMessage);
	const preview = $derived(message.trim() || DEFAULT_MESSAGE);

	async function saveSensitive(patch: { sensitiveMode?: boolean; sensitiveMessage?: string }, revert: () => void) {
		saving = true;
		clearTimeout(statusTimer);
		try {
			await adminSettings.update(patch);
			await invalidate('app:settings'); // the banner appears or changes straight away
			message = page.data.settings?.sensitiveMessage ?? message;
			status = 'saved';
		} catch {
			revert();
			status = 'error';
		} finally {
			saving = false;
			statusTimer = setTimeout(() => (status = null), status === 'saved' ? 2000 : 5000);
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

<div class="mt-5 rounded-xl border border-ink-800 bg-ink-900/60">
	<div class="border-b border-ink-800 px-5 py-4">
		<h3 class="text-sm font-semibold">Sensitive mode</h3>
	</div>
	<div class="space-y-4 px-5 py-4">
		<Toggle
			bind:checked={sensitive}
			disabled={saving}
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
					disabled={saving || !messageChanged}
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
				<span class="text-primary-300">Couldn't save. Try again.</span>
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

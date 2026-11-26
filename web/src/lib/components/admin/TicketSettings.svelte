<script lang="ts">
	import { onMount } from 'svelte';
	import { invalidate } from '$app/navigation';
	import { adminSettings, ApiError, getSettings, LENGTH_LIMITS, type Settings } from '$lib/api';
	import { ticketRef } from '$lib/tickets';
	import Toggle from '$lib/components/Toggle.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import CategoryManager from './CategoryManager.svelte';
	import TemplateManager from './TemplateManager.svelte';
	import SLAManager from './SLAManager.svelte';

	let settings = $state<Settings | null>(null);
	let loadFailed = $state(false);
	let saving = $state(false);
	let status = $state<'saved' | 'error' | null>(null);
	let statusTimer: ReturnType<typeof setTimeout>;

	// The prefix is typed, so it saves with a button rather than on every keystroke.
	let prefix = $state('');
	let prefixError = $state('');
	const PREFIX_RE = /^[A-Z0-9]{1,6}$/;
	const prefixClean = $derived(prefix.trim().toUpperCase());
	const prefixValid = $derived(PREFIX_RE.test(prefixClean));
	const prefixChanged = $derived(!!settings && prefixClean !== settings.ticketPrefix);

	// Length limits: typed numbers, saved together with a button.
	let subjectMax = $state(0);
	let descriptionMax = $state(0);
	let limitsError = $state('');
	const inRange = (n: number, r: { min: number; max: number }) => Number.isInteger(n) && n >= r.min && n <= r.max;
	const limitsValid = $derived(inRange(subjectMax, LENGTH_LIMITS.subject) && inRange(descriptionMax, LENGTH_LIMITS.description));
	const limitsChanged = $derived(
		!!settings && (subjectMax !== settings.subjectMaxLength || descriptionMax !== settings.descriptionMaxLength)
	);

	async function saveLimits(e: SubmitEvent) {
		e.preventDefault();
		if (!limitsValid) {
			limitsError = `Subject: ${LENGTH_LIMITS.subject.min}–${LENGTH_LIMITS.subject.max}. Description: ${LENGTH_LIMITS.description.min.toLocaleString()}–${LENGTH_LIMITS.description.max.toLocaleString()}.`;
			return;
		}
		limitsError = '';
		await save({ subjectMaxLength: subjectMax, descriptionMaxLength: descriptionMax }, () => {}, (err) => {
			limitsError = err instanceof ApiError && err.status === 400 ? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.' : '';
		});
		if (settings) {
			subjectMax = settings.subjectMaxLength;
			descriptionMax = settings.descriptionMaxLength;
		}
	}

	onMount(async () => {
		try {
			settings = await getSettings();
			prefix = settings.ticketPrefix;
			subjectMax = settings.subjectMaxLength;
			descriptionMax = settings.descriptionMaxLength;
		} catch {
			loadFailed = true;
		}
	});

	async function savePrefix(e: SubmitEvent) {
		e.preventDefault();
		if (!prefixValid) {
			prefixError = 'Use 1 to 6 letters or numbers.';
			return;
		}
		prefixError = '';
		await save({ ticketPrefix: prefixClean }, () => {}, (err) => {
			prefixError = err instanceof ApiError && err.status === 400 ? 'Use 1 to 6 letters or numbers.' : '';
		});
		if (settings) prefix = settings.ticketPrefix;
	}

	// Switches save as soon as they're changed; on failure they flip back.
	async function save(patch: Partial<Settings>, revert: () => void, onError?: (err: unknown) => void) {
		saving = true;
		clearTimeout(statusTimer);
		try {
			settings = await adminSettings.update(patch);
			status = 'saved';
			// Settings shape every page (e.g. ticket references), so refresh them app-wide.
			await invalidate('app:settings');
		} catch (err) {
			revert();
			onError?.(err);
			status = 'error';
		} finally {
			saving = false;
			statusTimer = setTimeout(() => (status = null), status === 'saved' ? 2000 : 5000);
		}
	}
</script>

<CategoryManager />
<TemplateManager />
<SLAManager />

<div class="mt-6 rounded-xl border border-ink-800 bg-ink-900/60">
	<div class="flex items-center justify-between gap-3 border-b border-ink-800 px-5 py-4">
		<h3 class="text-sm font-semibold">Ticket settings</h3>
		<span class="text-xs" aria-live="polite">
			{#if saving}
				<span class="text-ink-500">Saving…</span>
			{:else if status === 'saved'}
				<span class="flex items-center gap-1 text-ink-400"><Icon name="check" class="size-3.5 text-primary-300" /> Saved</span>
			{:else if status === 'error'}
				<span class="text-danger-300">Couldn't save. Try again.</span>
			{/if}
		</span>
	</div>
	<div class="px-5 py-4">
		{#if loadFailed}
			<p class="text-sm text-danger-200">Couldn't load settings.</p>
		{:else if !settings}
			<p class="text-sm text-ink-500">Loading…</p>
		{:else}
			<Toggle
				bind:checked={settings.requireCategory}
				disabled={saving}
				onchange={(on) => save({ requireCategory: on }, () => settings && (settings.requireCategory = !on))}
			>
				Mandatory category
				{#snippet description()}
					Users must choose a category when they raise a ticket. Only applies while at least one
					category below is active.
				{/snippet}
			</Toggle>
			<div class="my-4 h-px bg-ink-800"></div>
			<Toggle
				bind:checked={settings.allowImageUploads}
				disabled={saving}
				onchange={(on) => save({ allowImageUploads: on }, () => settings && (settings.allowImageUploads = !on))}
			>
				Allow image uploads to tickets
				{#snippet description()}
					Users can attach up to 5 images (PNG, JPG, GIF or WebP, 10 MB each) when they raise a ticket.
				{/snippet}
			</Toggle>
			<div class="my-4 h-px bg-ink-800"></div>
			<Toggle
				bind:checked={settings.allowRequesterReopen}
				disabled={saving}
				onchange={(on) => save({ allowRequesterReopen: on }, () => settings && (settings.allowRequesterReopen = !on))}
			>
				Allow reporters to reopen their tickets
				{#snippet description()}
					People can reopen their own tickets once they're resolved, closed or raised in error. When this is off,
					only agents and admins can reopen tickets, and reporters reply to ask instead.
				{/snippet}
			</Toggle>
			<div class="my-4 h-px bg-ink-800"></div>
			<Toggle
				bind:checked={settings.incidentTree}
				disabled={saving}
				onchange={(on) => save({ incidentTree: on }, () => settings && (settings.incidentTree = !on))}
			>
				Incident tree
				{#snippet description()}
					Staff can view a diagram of every ticket linked under an urgent ticket. Linking tickets works either way.
				{/snippet}
			</Toggle>
			<div class="my-4 h-px bg-ink-800"></div>
			<form onsubmit={savePrefix} novalidate class="flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
				<div class="min-w-0 flex-1">
					<label for="ticket-prefix" class="block text-sm font-medium text-ink-100">Ticket ID prefix</label>
					<p class="mt-0.5 text-sm text-ink-400">
						Shown before every ticket number. Up to 6 letters or numbers. Searching by an old prefix still works.
					</p>
				</div>
				<div class="flex shrink-0 flex-col items-end gap-1.5">
					<div class="flex items-center gap-2">
						<div class="relative">
							<input
								id="ticket-prefix"
								bind:value={prefix}
								oninput={() => (prefixError = '')}
								maxlength="6"
								autocomplete="off"
								spellcheck="false"
								aria-invalid={!!prefixError}
								aria-describedby="ticket-prefix-preview"
								class={['field h-9 w-28 pr-8 font-mono text-[13px] uppercase', prefixError && 'border-danger-400/70!']}
							/>
							<span class="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 font-mono text-[13px] text-ink-500">-</span>
						</div>
						<button
							type="submit"
							disabled={saving || !prefixChanged}
							class="h-9 rounded-lg bg-primary-400 px-3 text-[13px] font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-40"
						>
							Save
						</button>
					</div>
					<p id="ticket-prefix-preview" class="text-xs" aria-live="polite">
						{#if prefixError}
							<span class="text-danger-300">{prefixError}</span>
						{:else if prefixValid}
							<span class="text-ink-500">Looks like</span>
							<span class="font-mono text-ink-200">{ticketRef(1042, prefixClean)}</span>
						{:else}
							<span class="text-ink-500">Use 1 to 6 letters or numbers</span>
						{/if}
					</p>
				</div>
			</form>
			<div class="my-4 h-px bg-ink-800"></div>
			<form onsubmit={saveLimits} novalidate>
				<div class="flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
					<div class="min-w-0 flex-1">
						<p class="text-sm font-medium text-ink-100">Length limits</p>
						<p class="mt-0.5 text-sm text-ink-400">
							The most characters a new ticket's subject and description can have. Existing tickets aren't affected.
						</p>
					</div>
					<div class="flex shrink-0 flex-wrap items-end gap-2">
						<div>
							<label for="subject-max" class="mb-1 block text-xs text-ink-500">Subject</label>
							<input
								id="subject-max"
								type="number"
								inputmode="numeric"
								bind:value={subjectMax}
								oninput={() => (limitsError = '')}
								min={LENGTH_LIMITS.subject.min}
								max={LENGTH_LIMITS.subject.max}
								aria-describedby="limits-hint"
								class={['field h-9 w-24 text-[13px] tabular-nums', !inRange(subjectMax, LENGTH_LIMITS.subject) && 'border-danger-400/70!']}
							/>
						</div>
						<div>
							<label for="description-max" class="mb-1 block text-xs text-ink-500">Description</label>
							<input
								id="description-max"
								type="number"
								inputmode="numeric"
								step="100"
								bind:value={descriptionMax}
								oninput={() => (limitsError = '')}
								min={LENGTH_LIMITS.description.min}
								max={LENGTH_LIMITS.description.max}
								aria-describedby="limits-hint"
								class={['field h-9 w-28 text-[13px] tabular-nums', !inRange(descriptionMax, LENGTH_LIMITS.description) && 'border-danger-400/70!']}
							/>
						</div>
						<button
							type="submit"
							disabled={saving || !limitsChanged}
							class="h-9 rounded-lg bg-primary-400 px-3 text-[13px] font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-40"
						>
							Save
						</button>
					</div>
				</div>
				<p id="limits-hint" class="mt-2 text-right text-xs" aria-live="polite">
					{#if limitsError}
						<span class="text-danger-300">{limitsError}</span>
					{:else}
						<span class="text-ink-500">
							Subject {LENGTH_LIMITS.subject.min}–{LENGTH_LIMITS.subject.max} · Description
							{LENGTH_LIMITS.description.min.toLocaleString()}–{LENGTH_LIMITS.description.max.toLocaleString()} characters
						</span>
					{/if}
				</p>
			</form>
		{/if}
	</div>
</div>

<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { ApiError, backups, type Backup, type BackupSettings, type BackupStatus } from '$lib/api';
	import { dateTimeFmt, formatBytes, timeAgo } from '$lib/format';
	import Dropdown from '$lib/components/Dropdown.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import Toggle from '$lib/components/Toggle.svelte';

	let st = $state<BackupStatus | null>(null);
	let loadFailed = $state(false);

	// The schedule form, edited locally and saved with the button.
	let form = $state<BackupSettings>({ enabled: false, frequency: 'daily', hour: 2, keep: 7 });
	let saving = $state(false);
	let saveError = $state('');
	let saved = $state(false);
	const dirty = $derived(st !== null && JSON.stringify(form) !== JSON.stringify(st.settings));

	function show(next: BackupStatus) {
		st = next;
		form = { ...next.settings };
		if (next.running) poll();
	}

	onMount(async () => {
		try {
			show(await backups.get());
		} catch {
			loadFailed = true;
		}
	});

	// While a backup runs, check every 2 seconds until it's done.
	let timer: ReturnType<typeof setTimeout> | undefined;
	function poll() {
		clearTimeout(timer);
		timer = setTimeout(async () => {
			try {
				const next = await backups.get();
				st = next;
				if (next.running) poll();
			} catch {
				poll();
			}
		}, 2000);
	}
	onDestroy(() => clearTimeout(timer));

	const errorText = (err: unknown) =>
		err instanceof ApiError && err.status < 500
			? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.'
			: 'Something went wrong. Please try again.';

	async function saveSchedule(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		saveError = '';
		try {
			show(await backups.saveSettings(form));
			saved = true;
			setTimeout(() => (saved = false), 2000);
		} catch (err) {
			saveError = errorText(err);
		} finally {
			saving = false;
		}
	}

	let starting = $state(false);
	let startError = $state('');
	async function backUpNow() {
		starting = true;
		startError = '';
		try {
			await backups.create();
			if (st) st.running = true;
			poll();
		} catch (err) {
			startError = errorText(err);
		} finally {
			starting = false;
		}
	}

	let toDelete = $state<Backup | null>(null);
	let deleting = $state(false);
	let deleteError = $state('');
	const deleteOpen = {
		get: () => toDelete !== null,
		set: (open: boolean) => {
			if (!open && !deleting) toDelete = null;
		}
	};
	async function confirmDelete() {
		if (!toDelete) return;
		deleting = true;
		deleteError = '';
		try {
			await backups.delete(toDelete.name);
			if (st) st.backups = st.backups.filter((b) => b.name !== toDelete?.name);
			toDelete = null;
		} catch (err) {
			deleteError = errorText(err);
		} finally {
			deleting = false;
		}
	}

	const hourLabel = (h: number) => `${String(h).padStart(2, '0')}:00`;
	const hourOptions = Array.from({ length: 24 }, (_, h) => ({ value: h, label: hourLabel(h) }));
	const frequencyOptions = [
		{ value: 'daily' as const, label: 'Every day' },
		{ value: 'weekly' as const, label: 'Every week' }
	];

	const btnPrimary =
		'inline-flex h-9 items-center gap-1.5 rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-40';
	const btnSecondary =
		'inline-flex h-9 items-center rounded-lg border border-ink-700 px-3.5 text-sm font-medium text-ink-200 transition hover:border-ink-500 hover:text-ink-100 disabled:opacity-40';
</script>

{#if loadFailed}
	<p class="mt-5 text-sm text-danger-200">Couldn't load backups.</p>
{:else if !st}
	<p class="mt-5 text-sm text-ink-500">Loading…</p>
{:else}
	<!-- Backups and Back up now -->
	<div class="mt-5 rounded-xl border border-ink-800 bg-ink-900/60">
		<div class="flex flex-wrap items-start justify-between gap-3 border-b border-ink-800 px-5 py-4">
			<div class="min-w-0">
				<h3 class="text-sm font-semibold">Backups</h3>
				<p class="mt-0.5 text-sm text-ink-400">
					Each one holds the whole database and every uploaded file: attachments, knowledge base images, profile
					pictures and branding.
				</p>
			</div>
			<button type="button" onclick={backUpNow} disabled={st.running || starting} class={btnPrimary}>
				<Icon name={st.running ? 'refresh' : 'archive'} class={['size-4', st.running && 'animate-spin'].filter(Boolean).join(' ')} />
				{st.running ? 'Backing up…' : 'Back up now'}
			</button>
		</div>

		{#if startError}
			<p role="alert" class="border-b border-ink-800 px-5 py-2.5 text-sm text-danger-300">{startError}</p>
		{/if}
		{#if st.lastError && !st.running}
			<p role="alert" class="flex items-start gap-2 border-b border-ink-800 bg-danger-400/10 px-5 py-2.5 text-sm text-danger-200">
				<Icon name="shieldAlert" class="mt-0.5 size-4 shrink-0" />
				<span>The last backup failed: {st.lastError}</span>
			</p>
		{/if}

		{#if st.backups.length === 0}
			<p class="px-5 py-8 text-center text-sm text-ink-500">
				{st.running ? 'Making the first backup…' : 'No backups yet.'}
			</p>
		{:else}
			<ul class="divide-y divide-ink-800">
				{#each st.backups as b (b.name)}
					<li class="flex flex-wrap items-center gap-x-4 gap-y-1 px-5 py-3 text-sm">
						<span class="min-w-0 flex-1">
							<span class="block text-ink-100" title={b.name}>{dateTimeFmt.format(new Date(b.createdAt))}</span>
							<span class="block text-xs text-ink-500">{timeAgo(b.createdAt)} · {formatBytes(b.size)}</span>
						</span>
						<span
							class={[
								'rounded-md border px-2 py-0.5 text-xs',
								b.kind === 'scheduled' ? 'border-ink-700 text-ink-400' : 'border-primary-400/30 text-primary-200'
							]}
						>
							{b.kind === 'scheduled' ? 'Scheduled' : 'Manual'}
						</span>
						<span class="flex items-center gap-1">
							<a
								href={backups.downloadUrl(b.name)}
								download={b.name}
								class="inline-flex h-8 items-center gap-1.5 rounded-md px-2.5 text-[13px] text-ink-300 transition hover:bg-ink-850 hover:text-ink-100"
							>
								<Icon name="download" class="size-4" /> Download
							</a>
							<button
								type="button"
								onclick={() => {
									deleteError = '';
									toDelete = b;
								}}
								aria-label="Delete backup from {dateTimeFmt.format(new Date(b.createdAt))}"
								class="inline-flex size-8 items-center justify-center rounded-md text-ink-500 transition hover:bg-ink-850 hover:text-danger-300"
							>
								<Icon name="trash" class="size-4" />
							</button>
						</span>
					</li>
				{/each}
			</ul>
		{/if}

		<div class="space-y-1.5 border-t border-ink-800 px-5 py-3.5 text-xs text-ink-500">
			<p>
				Backups are kept on the server in <code class="font-mono text-[11px] text-ink-300">{st.folder}</code>, on the same
				disk as Lithium. They protect against mistakes, but not against losing that disk: download them regularly and
				keep copies somewhere else.
			</p>
			<p>A backup contains everything in Lithium, including password hashes and email settings. Store it securely.</p>
		</div>
	</div>

	<!-- Schedule -->
	<form onsubmit={saveSchedule} class="mt-6 rounded-xl border border-ink-800 bg-ink-900/60">
		<div class="flex items-center justify-between gap-3 border-b border-ink-800 px-5 py-4">
			<h3 class="text-sm font-semibold">Scheduled backups</h3>
			<span class="text-xs" aria-live="polite">
				{#if saved}
					<span class="flex items-center gap-1 text-ink-400"><Icon name="check" class="size-3.5 text-primary-300" /> Saved</span>
				{/if}
			</span>
		</div>
		<div class="space-y-5 px-5 py-4">
			<Toggle bind:checked={form.enabled} disabled={saving}>
				Back up automatically
				{#snippet description()}
					Lithium makes a backup on a schedule and deletes the oldest scheduled ones. Manual backups are never deleted
					automatically.
				{/snippet}
			</Toggle>

			{#if form.enabled}
				<div class="flex flex-wrap items-end gap-4">
					<div class="w-40">
						<label for="backup-frequency" class="mb-1 block text-xs text-ink-500">How often</label>
						<Dropdown id="backup-frequency" size="sm" bind:value={form.frequency} options={frequencyOptions} disabled={saving}>
							{#snippet item(opt)}<span class="text-[13px] text-ink-100">{opt.label}</span>{/snippet}
						</Dropdown>
					</div>
					<div class="w-32">
						<label for="backup-hour" class="mb-1 block text-xs text-ink-500">At ({st.timeZone})</label>
						<Dropdown id="backup-hour" size="sm" bind:value={form.hour} options={hourOptions} disabled={saving}>
							{#snippet item(opt)}<span class="font-mono text-[13px] text-ink-100 tabular-nums">{opt.label}</span>{/snippet}
						</Dropdown>
					</div>
					<div>
						<label for="backup-keep" class="mb-1 block text-xs text-ink-500">Keep the newest</label>
						<input
							id="backup-keep"
							type="number"
							inputmode="numeric"
							min="1"
							max="90"
							bind:value={form.keep}
							disabled={saving}
							class={['field h-9 w-20 text-[13px] tabular-nums', (form.keep < 1 || form.keep > 90) && 'border-danger-400/70!']}
						/>
					</div>
				</div>
				{#if st.settings.enabled && st.nextRun && !dirty}
					<p class="text-sm text-ink-400">
						Next backup: <span class="text-ink-200">{dateTimeFmt.format(new Date(st.nextRun))}</span>
					</p>
				{/if}
			{/if}

			{#if saveError}
				<p role="alert" class="text-sm text-danger-300">{saveError}</p>
			{/if}
			<div class="flex gap-2">
				<button type="submit" disabled={!dirty || saving || form.keep < 1 || form.keep > 90} class={btnPrimary}>
					{saving ? 'Saving…' : 'Save schedule'}
				</button>
				{#if dirty}
					<button type="button" onclick={() => st && (form = { ...st.settings })} disabled={saving} class={btnSecondary}>
						Cancel
					</button>
				{/if}
			</div>
		</div>
	</form>

	<!-- Restoring -->
	<div class="mt-6 rounded-xl border border-ink-800 bg-ink-900/60 px-5 py-4 text-sm">
		<h3 class="font-semibold">Restoring a backup</h3>
		<p class="mt-1 text-ink-400">
			Restoring replaces everything in Lithium, so it's done on the server rather than here. Stop Lithium, unpack the
			backup into the data folder (replacing <code class="font-mono text-[12px]">lithium.db</code> and deleting any
			<code class="font-mono text-[12px]">lithium.db-wal</code> and <code class="font-mono text-[12px]">lithium.db-shm</code>
			beside it), then start Lithium again. The README has the exact commands.
		</p>
	</div>
{/if}

<Modal
	bind:open={deleteOpen.get, deleteOpen.set}
	title="Delete this backup?"
	description={toDelete ? `The backup from ${dateTimeFmt.format(new Date(toDelete.createdAt))} is deleted from the server for good.` : ''}
>
	{#if deleteError}
		<p role="alert" class="mb-3 text-sm text-danger-300">{deleteError}</p>
	{/if}
	<div class="mt-2 flex justify-end gap-2">
		<button type="button" onclick={() => (toDelete = null)} disabled={deleting} class={btnSecondary}>Cancel</button>
		<button
			type="button"
			onclick={confirmDelete}
			disabled={deleting}
			class="inline-flex h-9 items-center rounded-lg bg-danger-600 px-3.5 text-sm font-semibold text-white transition hover:bg-danger-500 disabled:opacity-40"
		>
			{deleting ? 'Deleting…' : 'Delete backup'}
		</button>
	</div>
</Modal>

<script lang="ts">
	import { onMount } from 'svelte';
	import { ApiError, listCategories, slaRules, type Priority, type PublicCategory, type SLARule } from '$lib/api';
	import { formatMinutes } from '$lib/format';
	import { priorities, priorityLabel } from '$lib/tickets';
	import Dropdown, { type DropdownOption } from '$lib/components/Dropdown.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import PriorityIcon from '$lib/components/PriorityIcon.svelte';

	// SLA rules: response and resolution targets by category, priority or
	// both. The most specific matching rule applies to a ticket.
	let rules = $state<SLARule[] | null>(null);
	let categories = $state<PublicCategory[]>([]);
	let loadError = $state('');

	async function load() {
		try {
			[rules, categories] = await Promise.all([slaRules.list(), listCategories()]);
		} catch {
			loadError = "Couldn't load SLA rules.";
		}
	}
	onMount(load);

	const errorText = (err: unknown) =>
		err instanceof ApiError && err.status < 500
			? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.'
			: 'Something went wrong. Please try again.';

	// --- Durations: entered as a number and a unit ---
	type Unit = 'minutes' | 'hours' | 'days';
	const unitMinutes: Record<Unit, number> = { minutes: 1, hours: 60, days: 1440 };
	const unitOptions: DropdownOption<Unit>[] = [
		{ value: 'minutes', label: 'minutes' },
		{ value: 'hours', label: 'hours' },
		{ value: 'days', label: 'days' }
	];
	type Duration = { on: boolean; amount: number; unit: Unit };

	function toDuration(minutes: number | null, fallback: Duration): Duration {
		if (minutes === null) return { ...fallback, on: false };
		const unit: Unit = minutes % 1440 === 0 ? 'days' : minutes % 60 === 0 ? 'hours' : 'minutes';
		return { on: true, amount: minutes / unitMinutes[unit], unit };
	}
	const toMinutes = (d: Duration) => (d.on ? Math.round(d.amount * unitMinutes[d.unit]) : null);
	const validDuration = (d: Duration) => !d.on || (Number.isFinite(d.amount) && d.amount > 0 && toMinutes(d)! >= 1);

	// --- The add/edit dialog ---
	let editing = $state<SLARule | 'new' | null>(null);
	let categoryId = $state<number | ''>('');
	let priority = $state<Priority | ''>('');
	let first = $state<Duration>({ on: true, amount: 4, unit: 'hours' });
	let resolution = $state<Duration>({ on: true, amount: 2, unit: 'days' });
	let saving = $state(false);
	let formError = $state('');

	function openNew() {
		editing = 'new';
		categoryId = '';
		priority = '';
		first = { on: true, amount: 4, unit: 'hours' };
		resolution = { on: true, amount: 2, unit: 'days' };
		formError = '';
	}

	function openEdit(r: SLARule) {
		editing = r;
		categoryId = r.category?.id ?? '';
		priority = r.priority;
		first = toDuration(r.firstResponseMinutes, { on: false, amount: 4, unit: 'hours' });
		resolution = toDuration(r.resolutionMinutes, { on: false, amount: 2, unit: 'days' });
		formError = '';
	}

	const dialogOpen = {
		get: () => editing !== null,
		set: (open: boolean) => {
			if (!open && !saving) editing = null;
		}
	};

	async function save(e: SubmitEvent) {
		e.preventDefault();
		if (!first.on && !resolution.on) {
			formError = 'Set at least one target.';
			return;
		}
		if (!validDuration(first) || !validDuration(resolution)) {
			formError = 'Targets must be more than zero.';
			return;
		}
		saving = true;
		formError = '';
		const input = {
			categoryId: categoryId === '' ? null : categoryId,
			priority,
			firstResponseMinutes: toMinutes(first),
			resolutionMinutes: toMinutes(resolution)
		};
		try {
			if (editing === 'new') await slaRules.create(input);
			else if (editing) await slaRules.update(editing.id, input);
			rules = await slaRules.list(); // keeps the server's order
			editing = null;
		} catch (err) {
			formError = errorText(err);
		} finally {
			saving = false;
		}
	}

	// --- Deleting ---
	let deleting = $state<SLARule | null>(null);
	let deleteError = $state('');
	let removing = $state(false);
	const deleteOpen = {
		get: () => deleting !== null,
		set: (open: boolean) => {
			if (!open && !removing) deleting = null;
		}
	};
	async function confirmDelete() {
		if (!deleting) return;
		removing = true;
		deleteError = '';
		try {
			await slaRules.delete(deleting.id);
			rules = rules?.filter((r) => r.id !== deleting?.id) ?? null;
			deleting = null;
		} catch (err) {
			deleteError = errorText(err);
		} finally {
			removing = false;
		}
	}

	// --- Labels ---
	function ruleName(r: Pick<SLARule, 'category' | 'priority'>) {
		if (!r.category && !r.priority) return 'All other tickets';
		if (!r.category) return `${priorityLabel(r.priority as Priority)} priority, any category`;
		if (!r.priority) return `${r.category.name}, any priority`;
		return `${r.category.name}, ${priorityLabel(r.priority)} priority`;
	}
	function targets(r: SLARule) {
		const parts = [];
		if (r.firstResponseMinutes) parts.push(`First response ${formatMinutes(r.firstResponseMinutes)}`);
		if (r.resolutionMinutes) parts.push(`Resolve ${formatMinutes(r.resolutionMinutes)}`);
		return parts.join(' · ');
	}

	const categoryOptions = $derived<DropdownOption<number | ''>[]>([
		{ value: '', label: 'Any category' },
		...categories.map((c) => ({ value: c.id, label: c.name }))
	]);
	const categoryById = $derived(new Map(categories.map((c) => [c.id, c])));
	const priorityOptions: DropdownOption<Priority | ''>[] = [
		{ value: '', label: 'Any priority' },
		...priorities.map((p) => ({ value: p.value, label: p.label }))
	];

	const iconBtn =
		'flex size-8 items-center justify-center rounded-md text-ink-500 transition hover:bg-ink-850 hover:text-ink-100';
	const btnSecondary =
		'inline-flex h-9 items-center rounded-lg border border-ink-700 px-3.5 text-sm font-medium text-ink-200 transition hover:border-ink-500 hover:text-ink-100 disabled:opacity-40';
	const btnPrimary =
		'inline-flex h-9 items-center rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:opacity-40';
</script>

{#snippet durationField(id: string, label: string, hint: string, d: Duration)}
	<div class="rounded-lg border border-ink-800 px-3.5 py-3">
		<label class="flex cursor-pointer items-start gap-2.5">
			<input type="checkbox" bind:checked={d.on} disabled={saving} class="mt-0.5 size-4 shrink-0 accent-primary-400" />
			<span>
				<span class="block text-sm font-medium text-ink-100">{label}</span>
				<span class="block text-xs text-ink-500">{hint}</span>
			</span>
		</label>
		{#if d.on}
			<div class="mt-3 flex items-center gap-2 pl-6.5">
				<input
					{id}
					type="number"
					inputmode="decimal"
					min="1"
					step="any"
					bind:value={d.amount}
					disabled={saving}
					aria-label="{label}: amount"
					class={['field h-9 w-24 text-[13px] tabular-nums', !validDuration(d) && 'border-danger-400/70!']}
				/>
				<div class="w-32">
					<Dropdown id="{id}-unit" size="sm" label="{label}: unit" bind:value={d.unit} options={unitOptions} disabled={saving}>
						{#snippet item(opt)}<span class="text-[13px] text-ink-100">{opt.label}</span>{/snippet}
					</Dropdown>
				</div>
			</div>
		{/if}
	</div>
{/snippet}

<div class="mt-6 rounded-xl border border-ink-800 bg-ink-900/60">
	<div class="flex items-center justify-between gap-3 border-b border-ink-800 px-5 py-4">
		<div>
			<h3 class="text-sm font-semibold">SLAs</h3>
			<p class="mt-0.5 text-sm text-ink-400">
				Targets for the first reply and for resolving tickets. Admins are warned when a target is close and when it's
				missed.
			</p>
		</div>
		<button
			type="button"
			onclick={openNew}
			disabled={rules === null}
			class="flex h-8 shrink-0 items-center gap-1.5 rounded-lg bg-primary-400 px-3 text-[13px] font-semibold text-primary-950 transition hover:bg-primary-300 disabled:opacity-40"
		>
			<Icon name="plus" class="size-4" /> New rule
		</button>
	</div>

	{#if loadError}
		<p class="px-5 py-10 text-center text-sm text-danger-200">{loadError}</p>
	{:else if rules === null}
		<p class="px-5 py-10 text-center text-sm text-ink-500">Loading SLA rules…</p>
	{:else if rules.length === 0}
		<div class="flex flex-col items-center px-6 py-10 text-center">
			<span class="flex size-11 items-center justify-center rounded-xl bg-primary-400/10 text-primary-400">
				<Icon name="activity" class="size-5" />
			</span>
			<p class="mt-4 text-sm font-medium text-ink-200">No SLA rules yet</p>
			<p class="mt-1 max-w-sm text-sm text-ink-500">
				For example: urgent tickets get a first response within 1 hour and are resolved within 4 hours.
			</p>
		</div>
	{:else}
		<ul class="divide-y divide-ink-800 p-1.5">
			{#each rules as r (r.id)}
				<li class="flex items-center gap-3 rounded-lg px-3.5 py-3">
					<span class="flex w-5 shrink-0 justify-center text-ink-400">
						{#if r.priority}<PriorityIcon priority={r.priority} class="size-4" />{/if}
					</span>
					<div class="min-w-0 flex-1">
						<p class="flex min-w-0 items-center gap-2 text-sm font-medium text-ink-100">
							{#if r.category}<span class="tag-{r.category.color} flex size-3 shrink-0 items-center justify-center"><span class="tag-dot size-2.5 rounded-full"></span></span>{/if}
							<span class="truncate">{ruleName(r)}</span>
						</p>
						<p class="truncate text-xs text-ink-500">{targets(r)}</p>
					</div>
					<div class="flex shrink-0 items-center">
						<button type="button" onclick={() => openEdit(r)} class={iconBtn} aria-label="Edit {ruleName(r)}" title="Edit">
							<Icon name="pencil" class="size-4" />
						</button>
						<button
							type="button"
							onclick={() => {
								deleteError = '';
								deleting = r;
							}}
							class={iconBtn}
							aria-label="Delete {ruleName(r)}"
							title="Delete"
						>
							<Icon name="trash" class="size-4" />
						</button>
					</div>
				</li>
			{/each}
		</ul>
	{/if}

	<p class="border-t border-ink-800 px-5 py-3 text-xs text-ink-500">
		When several rules match a ticket, the most specific wins: category and priority, then category, then priority, then
		“all other tickets”. The clock counts every hour and pauses while a ticket is Pending. First response is the first
		reply from an agent or admin.
	</p>
</div>

<Modal
	bind:open={dialogOpen.get, dialogOpen.set}
	title={editing === 'new' ? 'New SLA rule' : 'Edit SLA rule'}
	description="Choose which tickets it covers, then the targets they should meet."
>
	<form onsubmit={save} class="space-y-4">
		<div class="grid gap-3 sm:grid-cols-2">
			<div>
				<label for="sla-category" class="mb-1 block text-xs text-ink-500">Category</label>
				<Dropdown id="sla-category" size="sm" bind:value={categoryId} options={categoryOptions} disabled={saving} searchable={categories.length > 8}>
					{#snippet item(opt)}
						{@const c = opt.value === '' ? null : categoryById.get(opt.value)}
						{#if c}
							<span class="tag-{c.color} flex size-3 shrink-0 items-center justify-center"><span class="tag-dot size-2.5 rounded-full"></span></span>
							<span class="truncate text-[13px] text-ink-100">{c.name}</span>
						{:else}
							<Icon name="tag" class="size-4 shrink-0 text-ink-500" />
							<span class="text-[13px] text-ink-400">Any category</span>
						{/if}
					{/snippet}
				</Dropdown>
			</div>
			<div>
				<label for="sla-priority" class="mb-1 block text-xs text-ink-500">Priority</label>
				<Dropdown id="sla-priority" size="sm" bind:value={priority} options={priorityOptions} disabled={saving}>
					{#snippet item(opt)}
						{#if opt.value}
							<PriorityIcon priority={opt.value} class="size-4 shrink-0 text-ink-400" />
							<span class="text-[13px] text-ink-100">{opt.label}</span>
						{:else}
							<Icon name="chart" class="size-4 shrink-0 text-ink-500" />
							<span class="text-[13px] text-ink-400">Any priority</span>
						{/if}
					{/snippet}
				</Dropdown>
			</div>
		</div>
		<p class="text-xs text-ink-500">Applies to: <span class="text-ink-300">{ruleName({ category: categoryId === '' ? null : (categoryById.get(categoryId) ?? null), priority })}</span></p>

		{@render durationField('sla-first', 'First response', 'Time until an agent or admin first replies.', first)}
		{@render durationField('sla-resolution', 'Resolution', 'Time until the ticket is resolved or closed.', resolution)}

		{#if formError}
			<p role="alert" class="text-sm text-danger-300">{formError}</p>
		{/if}
		<div class="flex justify-end gap-2">
			<button type="button" onclick={() => (editing = null)} disabled={saving} class={btnSecondary}>Cancel</button>
			<button type="submit" disabled={saving} class={btnPrimary}>{saving ? 'Saving…' : editing === 'new' ? 'Add rule' : 'Save rule'}</button>
		</div>
	</form>
</Modal>

<Modal
	bind:open={deleteOpen.get, deleteOpen.set}
	title="Delete this SLA rule?"
	description={deleting ? `${ruleName(deleting)} will no longer have these targets. Tickets fall back to the next matching rule, if any.` : ''}
>
	{#if deleteError}
		<p role="alert" class="mb-3 text-sm text-danger-300">{deleteError}</p>
	{/if}
	<div class="mt-2 flex justify-end gap-2">
		<button type="button" onclick={() => (deleting = null)} disabled={removing} class={btnSecondary}>Cancel</button>
		<button
			type="button"
			onclick={confirmDelete}
			disabled={removing}
			class="inline-flex h-9 items-center rounded-lg bg-danger-600 px-3.5 text-sm font-semibold text-white transition hover:bg-danger-500 disabled:opacity-40"
		>
			{removing ? 'Deleting…' : 'Delete rule'}
		</button>
	</div>
</Modal>

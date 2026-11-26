<script lang="ts">
	import { onMount } from 'svelte';
	import {
		ApiError,
		searchUsers,
		tickets,
		type PublicCategory,
		type TicketChanges,
		type TicketStatus,
		type UserRef
	} from '$lib/api';
	import { priorities, statusLabels } from '$lib/tickets';
	import Avatar from './Avatar.svelte';
	import Dropdown, { type DropdownOption } from './Dropdown.svelte';
	import Icon from './Icon.svelte';
	import Modal from './Modal.svelte';
	import PriorityIcon from './PriorityIcon.svelte';
	import StatusBadge from './StatusBadge.svelte';

	// The bar along the bottom of the tickets list in select mode. Picking a
	// change asks for confirmation, then applies it to every selected ticket.
	let {
		ids,
		categories,
		me,
		onclear,
		ondone
	}: {
		ids: number[];
		categories: PublicCategory[];
		me: number | undefined;
		onclear: () => void;
		/** After a change: how many updated, and why any were skipped. */
		ondone: (result: { updated: number; skipped: { id: number; reason: string }[]; what: string }) => void;
	} = $props();

	let staff = $state<UserRef[]>([]);
	onMount(async () => {
		try {
			// You first, then everyone else alphabetically (as the server returns them).
			staff = (await searchUsers('', { staff: true })).sort((a, b) => Number(b.id === me) - Number(a.id === me));
		} catch {
			/* the assignee menu just shows "Unassigned" */
		}
	});

	// Option values carry the change; "none" clears a category or assignee.
	const assigneeOptions = $derived<DropdownOption<string>[]>([
		{ value: 'none', label: 'Unassigned' },
		...staff.map((u) => ({ value: String(u.id), label: u.name, keywords: u.email }))
	]);
	const staffById = $derived(new Map(staff.map((u) => [String(u.id), u])));
	const statusOptions = (Object.keys(statusLabels) as TicketStatus[]).map((s) => ({ value: s, label: statusLabels[s] }));
	const priorityOptions = priorities.map((p) => ({ value: p.value, label: p.label }));
	const categoryOptions = $derived<DropdownOption<string>[]>([
		{ value: 'none', label: 'No category' },
		...categories.map((c) => ({ value: String(c.id), label: c.name }))
	]);
	const categoryById = $derived(new Map(categories.map((c) => [String(c.id), c])));

	// The change waiting for confirmation.
	let pending = $state<{ changes: TicketChanges; what: string } | null>(null);
	let applying = $state(false);
	let error = $state('');

	// Bumped after each pick so the menus go back to their placeholders.
	let menus = $state(0);

	function ask(changes: TicketChanges, what: string) {
		error = '';
		pending = { changes, what };
		menus++;
	}

	const confirmOpen = {
		get: () => pending !== null,
		set: (open: boolean) => {
			if (!open && !applying) pending = null;
		}
	};

	async function apply() {
		if (!pending) return;
		applying = true;
		error = '';
		try {
			const res = await tickets.bulk(ids, pending.changes);
			ondone({ ...res, what: pending.what });
			pending = null;
		} catch (err) {
			error =
				err instanceof ApiError && err.status < 500
					? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.'
					: 'Something went wrong. Please try again.';
		} finally {
			applying = false;
		}
	}

	const n = $derived(ids.length);
	const plural = (k: number) => `${k} ${k === 1 ? 'ticket' : 'tickets'}`;
</script>

<div
	role="toolbar"
	aria-label="Bulk actions"
	class="fixed inset-x-0 bottom-4 z-40 mx-auto flex w-fit max-w-[calc(100vw-2rem)] flex-wrap items-center gap-2 rounded-xl border border-ink-700 bg-ink-900 p-2 shadow-2xl shadow-shade/40"
>
	<span class="px-2 text-sm font-medium whitespace-nowrap text-ink-100">{n} selected</span>

	{#key menus}

	<div class="w-40">
		<Dropdown
			id="bulk-assignee"
			size="sm"
			label="Assign selected tickets"
			placement="above"
			menu="wide"
			value=""
			placeholder="Assign…"
			disabled={n === 0}
			searchable={staff.length > 6}
			options={assigneeOptions}
			onchange={(v) =>
				ask(
					{ assigneeId: v === 'none' ? null : Number(v) },
					v === 'none' ? 'Unassign' : `Assign to ${staffById.get(v)?.name ?? 'them'}`
				)}
		>
			{#snippet item(opt)}
				{@const u = staffById.get(opt.value)}
				{#if u}
					<Avatar name={u.name} src={u.avatarUrl} size={20} tone={u.id === me ? 'soft' : 'neutral'} />
					<span class="truncate text-[13px] text-ink-100">{u.name}{#if u.id === me}<span class="ml-1 text-ink-500">(you)</span>{/if}</span>
				{:else}
					<span class="size-5 shrink-0 rounded-full border border-dashed border-ink-500"></span>
					<span class="text-[13px] text-ink-400">Unassigned</span>
				{/if}
			{/snippet}
		</Dropdown>
	</div>

	<div class="w-36">
		<Dropdown
			id="bulk-status"
			size="sm"
			label="Change status of selected tickets"
			placement="above"
			menu="wide"
			value=""
			placeholder="Status…"
			disabled={n === 0}
			options={statusOptions}
			onchange={(v) => ask({ status: v as TicketStatus }, `Change status to ${statusLabels[v as TicketStatus]}`)}
		>
			{#snippet item(opt)}<StatusBadge status={opt.value as TicketStatus} />{/snippet}
		</Dropdown>
	</div>

	<div class="w-36">
		<Dropdown
			id="bulk-priority"
			size="sm"
			label="Change priority of selected tickets"
			placement="above"
			value=""
			placeholder="Priority…"
			disabled={n === 0}
			options={priorityOptions}
			onchange={(v) => {
				const p = priorities.find((x) => x.value === v);
				if (p) ask({ priority: p.value }, `Change priority to ${p.label}`);
			}}
		>
			{#snippet item(opt)}
				<PriorityIcon priority={opt.value as (typeof priorities)[number]['value']} class="size-4 shrink-0 text-ink-400" />
				<span class="text-[13px] text-ink-100">{opt.label}</span>
			{/snippet}
		</Dropdown>
	</div>

	{#if categories.length > 0}
		<div class="w-40">
			<Dropdown
				id="bulk-category"
				size="sm"
				label="Change category of selected tickets"
				placement="above"
				menu="wide"
				value=""
				placeholder="Category…"
				disabled={n === 0}
				searchable={categories.length > 8}
				options={categoryOptions}
				onchange={(v) =>
					ask(
						{ categoryId: v === 'none' ? null : Number(v) },
						v === 'none' ? 'Remove the category' : `Change category to ${categoryById.get(v)?.name ?? 'it'}`
					)}
			>
				{#snippet item(opt)}
					{@const c = categoryById.get(opt.value)}
					{#if c}
						<span class="tag-{c.color} flex size-3 shrink-0 items-center justify-center"><span class="tag-dot size-2.5 rounded-full"></span></span>
						<span class="truncate text-[13px] text-ink-100">{c.name}</span>
					{:else}
						<Icon name="tag" class="size-4 shrink-0 text-ink-500" />
						<span class="text-[13px] text-ink-400">No category</span>
					{/if}
				{/snippet}
			</Dropdown>
		</div>
	{/if}
	{/key}

	<button
		type="button"
		onclick={onclear}
		disabled={n === 0}
		class="h-8 rounded-lg px-2.5 text-[13px] text-ink-400 transition hover:bg-ink-850 hover:text-ink-100 disabled:opacity-40"
	>
		Clear
	</button>
</div>

<Modal
	bind:open={confirmOpen.get, confirmOpen.set}
	title={pending ? `${pending.what} for ${plural(n)}?` : ''}
	description="Each ticket records the change in its conversation, and the usual people are notified."
>
	{#if error}
		<p role="alert" class="mb-3 text-sm text-danger-300">{error}</p>
	{/if}
	<div class="mt-2 flex justify-end gap-2">
		<button
			type="button"
			onclick={() => (pending = null)}
			disabled={applying}
			class="inline-flex h-9 items-center rounded-lg border border-ink-700 px-3.5 text-sm font-medium text-ink-200 transition hover:border-ink-500 hover:text-ink-100"
		>
			Cancel
		</button>
		<button
			type="button"
			onclick={apply}
			disabled={applying}
			class="inline-flex h-9 items-center rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:opacity-40"
		>
			{applying ? 'Updating…' : `Update ${plural(n)}`}
		</button>
	</div>
</Modal>

<script lang="ts">
	import { page } from '$app/state';
	import { tickets, type TicketSummary } from '$lib/api';
	import { ticketRef } from '$lib/tickets';
	import Icon from './Icon.svelte';
	import Modal from './Modal.svelte';
	import PriorityIcon from './PriorityIcon.svelte';
	import StatusBadge from './StatusBadge.svelte';

	// Search for a ticket by subject, requester or reference, and pick one.
	let {
		open = $bindable(false),
		title,
		description,
		exclude = [],
		onpick
	}: {
		open?: boolean;
		title: string;
		description?: string;
		/** Tickets that can't be picked (e.g. this one, already linked ones). */
		exclude?: number[];
		onpick: (t: TicketSummary) => void;
	} = $props();

	const prefix = $derived(page.data.settings?.ticketPrefix ?? 'TID');
	let query = $state('');
	let results = $state<TicketSummary[]>([]);
	let loading = $state(false);
	let seq = 0;

	$effect(() => {
		if (open) query = '';
	});

	$effect(() => {
		if (!open) return;
		const q = query.trim();
		const t = setTimeout(async () => {
			const id = ++seq;
			loading = true;
			try {
				const res = await tickets.list({ q, limit: 12 });
				if (id === seq) results = res.tickets.filter((r) => !exclude.includes(r.id));
			} catch {
				if (id === seq) results = [];
			} finally {
				if (id === seq) loading = false;
			}
		}, q ? 200 : 0);
		return () => clearTimeout(t);
	});
</script>

<Modal bind:open {title} {description}>
	<div class="relative">
		<Icon name="search" class="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-500" />
		<!-- svelte-ignore a11y_autofocus -->
		<input
			type="search"
			bind:value={query}
			autofocus
			placeholder="Search subject, requester or {prefix}-42"
			aria-label="Search tickets"
			class="field pl-9"
		/>
	</div>
	<ul class="mt-3 max-h-80 space-y-1 overflow-y-auto">
		{#each results as r (r.id)}
			<li>
				<button
					type="button"
					onclick={() => {
						open = false;
						onpick(r);
					}}
					class="flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left transition hover:bg-ink-850 focus-visible:bg-ink-850 focus-visible:outline-none"
				>
					<PriorityIcon priority={r.priority} class="size-4 shrink-0 text-ink-400" />
					<span class="min-w-0 flex-1">
						<span class="block truncate text-sm text-ink-100">{r.subject}</span>
						<span class="block text-xs text-ink-500"><span class="font-mono">{ticketRef(r.id, prefix)}</span> · {r.requester.name}</span>
					</span>
					<StatusBadge status={r.status} />
				</button>
			</li>
		{:else}
			<li class="px-3 py-8 text-center text-sm text-ink-500">
				{loading ? 'Searching…' : query ? 'No matching tickets' : 'No tickets to link'}
			</li>
		{/each}
	</ul>
</Modal>

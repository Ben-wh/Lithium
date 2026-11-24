<script lang="ts">
	import type { TicketSummary } from '$lib/api';
	import { timeAgo } from '$lib/format';
	import { page } from '$app/state';
	import { priorityLabel, ticketRef } from '$lib/tickets';
	import CategoryTag from './CategoryTag.svelte';
	import Avatar from './Avatar.svelte';
	import PriorityIcon from './PriorityIcon.svelte';
	import StatusBadge from './StatusBadge.svelte';

	let { ticket: t, showRequester = false }: { ticket: TicketSummary; showRequester?: boolean } = $props();
	const prefix = $derived(page.data.settings?.ticketPrefix ?? 'TID');
	const ref = $derived(ticketRef(t.id, prefix));
</script>

<a
	href="/tickets/{t.id}"
	class={[
		'group flex items-center gap-3 rounded-lg px-2.5 py-3 transition hover:bg-ink-850/60 focus-visible:bg-ink-850/60 focus-visible:outline-none sm:gap-4 sm:px-3.5',
		// Urgent tickets get a red outline. Inset so it doesn't shift the layout or overlap the row dividers.
		t.priority === 'urgent' && 'bg-danger-400/5 ring-1 ring-danger-300/80 ring-inset'
	]}
>
	<span title="{priorityLabel(t.priority)} priority" class="shrink-0 text-ink-400">
		<PriorityIcon priority={t.priority} class="size-4" />
		<span class="sr-only">{priorityLabel(t.priority)} priority</span>
	</span>
	<!-- Fixed width (prefix + "-" + 4 digits) so subjects line up down the list. -->
	<span class="hidden shrink-0 font-mono text-xs text-ink-500 sm:block" style:width="{prefix.length + 5}ch">{ref}</span>

	<span class="min-w-0 flex-1">
		<span class="flex min-w-0 items-center gap-2">
			<span class="truncate text-sm font-medium text-ink-100">{t.subject}</span>
			{#if t.category}<CategoryTag category={t.category} class="hidden sm:inline-flex" />{/if}
		</span>
		<span class="block truncate text-xs text-ink-500">
			<span class="sm:hidden">{ref} · </span>{showRequester ? `${t.requester.name} · ` : ''}{#if t.category}<span class="sm:hidden">{t.category.name} · </span>{/if}updated {timeAgo(t.updatedAt)}
		</span>
	</span>

	<StatusBadge status={t.status} />

	{#if t.assignee}
		<span title="Assigned to {t.assignee.name}" class="shrink-0">
			<Avatar name={t.assignee.name} src={t.assignee.avatarUrl} size={28} />
			<span class="sr-only">Assigned to {t.assignee.name}</span>
		</span>
	{:else}
		<span title="Unassigned" class="size-7 shrink-0 rounded-full border border-dashed border-ink-700">
			<span class="sr-only">Unassigned</span>
		</span>
	{/if}
</a>

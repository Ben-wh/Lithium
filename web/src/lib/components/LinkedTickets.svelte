<script lang="ts">
	import { page } from '$app/state';
	import { ApiError, tickets, type TicketLink, type TicketLinks, type TicketSummary } from '$lib/api';
	import { priorityLabel, statusLabels, ticketRef } from '$lib/tickets';
	import Icon from './Icon.svelte';
	import PriorityIcon from './PriorityIcon.svelte';
	import TicketPicker from './TicketPicker.svelte';

	// Staff panel for linking a ticket into a tree: one parent, any children.
	let {
		ticketId,
		links = $bindable(),
		onchange
	}: {
		ticketId: number;
		links: TicketLinks;
		/** Called after a link changes (e.g. to refresh the activity thread). */
		onchange?: () => void;
	} = $props();

	const prefix = $derived(page.data.settings?.ticketPrefix ?? 'TID');
	let picking = $state<'parent' | 'child' | null>(null);
	let pickerOpen = $state(false);
	let busy = $state(false);
	let error = $state('');

	const exclude = $derived([ticketId, ...(links.parent ? [links.parent.id] : []), ...links.children.map((c) => c.id)]);

	async function run(action: () => Promise<unknown>) {
		busy = true;
		error = '';
		try {
			await action();
			links = await tickets.links(ticketId);
			onchange?.();
		} catch (err) {
			error =
				err instanceof ApiError && err.status < 500
					? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.'
					: "Couldn't update links. Please try again.";
		} finally {
			busy = false;
		}
	}

	function pick(t: TicketSummary) {
		if (picking === 'parent') run(() => tickets.setParent(ticketId, t.id));
		else run(() => tickets.setParent(t.id, ticketId));
	}

	function openPicker(kind: 'parent' | 'child') {
		picking = kind;
		pickerOpen = true;
	}

	const addBtn =
		'flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-xs font-medium text-primary-300 transition hover:bg-ink-850 hover:text-primary-200 disabled:opacity-50';
</script>

{#snippet row(t: TicketLink, unlink: () => void)}
	<li class="group flex items-center gap-2 rounded-md px-2 py-1.5 transition hover:bg-ink-850">
		<PriorityIcon priority={t.priority} class="size-3.5 shrink-0 text-ink-400" />
		<a
			href="/tickets/{t.id}"
			title="{priorityLabel(t.priority)} · {statusLabels[t.status]}"
			class="min-w-0 flex-1 truncate text-[13px] text-ink-200 hover:text-ink-100"
		>
			<span class="font-mono text-xs text-ink-500">{ticketRef(t.id, prefix)}</span>
			{t.subject}
		</a>
		<button
			type="button"
			onclick={unlink}
			disabled={busy}
			aria-label="Unlink {ticketRef(t.id, prefix)}"
			title="Unlink"
			class="flex size-6 shrink-0 items-center justify-center rounded text-ink-500 opacity-0 transition group-hover:opacity-100 hover:text-primary-300 focus-visible:opacity-100 max-sm:opacity-100"
		>
			<Icon name="close" class="size-3.5" />
		</button>
	</li>
{/snippet}

<div>
	<p class="mb-1 flex items-center gap-1.5 text-xs text-ink-500"><Icon name="link" class="size-3.5" /> Linked tickets</p>

	{#if error}
		<p role="alert" class="my-1.5 text-xs text-primary-300">{error}</p>
	{/if}

	<p class="mt-2 px-2 text-[11px] font-medium tracking-wide text-ink-500 uppercase">Part of</p>
	{#if links.parent}
		<ul class="mt-0.5">{@render row(links.parent, () => run(() => tickets.setParent(ticketId, null)))}</ul>
	{:else}
		<button type="button" disabled={busy} onclick={() => openPicker('parent')} class={addBtn}>
			<Icon name="plus" class="size-3.5" /> Link to a parent ticket
		</button>
	{/if}

	<p class="mt-2 px-2 text-[11px] font-medium tracking-wide text-ink-500 uppercase">
		Linked under this{#if links.children.length}<span class="ml-1 normal-case">· {links.children.length}</span>{/if}
	</p>
	{#if links.children.length}
		<ul class="mt-0.5">
			{#each links.children as c (c.id)}
				{@render row(c, () => run(() => tickets.setParent(c.id, null)))}
			{/each}
		</ul>
	{/if}
	<button type="button" disabled={busy} onclick={() => openPicker('child')} class={addBtn}>
		<Icon name="plus" class="size-3.5" /> Add a linked ticket
	</button>

	{#if links.incidentRootId}
		<a
			href="/tickets/{links.incidentRootId}/tree"
			class="mt-3 flex items-center justify-center gap-1.5 rounded-lg border border-primary-400/40 bg-primary-400/10 px-3 py-2 text-[13px] font-medium text-primary-200 transition hover:bg-primary-400/15"
		>
			<Icon name="tree" class="size-4" /> View incident tree
		</a>
	{/if}
</div>

<TicketPicker
	bind:open={pickerOpen}
	title={picking === 'parent' ? 'Link to a parent ticket' : 'Add a linked ticket'}
	description={picking === 'parent'
		? `${ticketRef(ticketId, prefix)} will appear under the ticket you pick.`
		: `The ticket you pick will appear under ${ticketRef(ticketId, prefix)}.`}
	{exclude}
	onpick={pick}
/>

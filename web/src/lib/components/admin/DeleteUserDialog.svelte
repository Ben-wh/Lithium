<script lang="ts">
	import { ApiError, admin, type AdminUser } from '$lib/api';
	import Avatar from '$lib/components/Avatar.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import Modal from '$lib/components/Modal.svelte';

	// Deleting an account takes two deliberate steps: first a plain summary
	// of what will be lost, then typing the user's email and ticking a box.
	let {
		open = $bindable(false),
		user,
		ondeleted
	}: {
		open?: boolean;
		user: AdminUser | null;
		ondeleted: (user: AdminUser, ticketsDeleted: number) => void;
	} = $props();

	let step = $state<1 | 2>(1);
	let typed = $state('');
	let understood = $state(false);
	let deleting = $state(false);
	let error = $state('');

	// Start from the beginning every time the dialog opens.
	$effect(() => {
		if (open) {
			step = 1;
			typed = '';
			understood = false;
			error = '';
		}
	});

	const plural = (n: number, word: string) => `${n.toLocaleString()} ${word}${n === 1 ? '' : 's'}`;
	const emailMatches = $derived(!!user && typed.trim().toLowerCase() === user.email.toLowerCase());
	const canDelete = $derived(emailMatches && understood && !deleting);

	async function confirmDelete(e: SubmitEvent) {
		e.preventDefault();
		if (!user || !canDelete) return;
		deleting = true;
		error = '';
		try {
			const { ticketsDeleted } = await admin.deleteUser(user.id);
			open = false;
			ondeleted(user, ticketsDeleted);
		} catch (err) {
			error =
				err instanceof ApiError && err.status < 500
					? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.'
					: 'Something went wrong. The account was not deleted.';
		} finally {
			deleting = false;
		}
	}

	const dangerBtn =
		'inline-flex h-9 items-center gap-1.5 rounded-lg bg-primary-600 px-3.5 text-sm font-semibold text-white transition hover:bg-primary-500 disabled:cursor-not-allowed disabled:opacity-40';
	const ghostBtn = 'h-9 rounded-lg px-3.5 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100';
</script>

<Modal
	bind:open
	title={step === 1 ? 'Delete this account?' : 'Are you absolutely sure?'}
	description={step === 1 ? 'Check what will be lost before you continue.' : "This can't be undone."}
>
	{#if user}
		<div class="mb-5 flex items-center gap-3 rounded-lg border border-ink-800 bg-ink-950/50 px-3 py-2.5">
			<Avatar name={user.name} src={user.avatarUrl} size={32} />
			<div class="min-w-0">
				<p class="truncate text-sm font-medium text-ink-100">{user.name}</p>
				<p class="truncate text-xs text-ink-400">{user.email}</p>
			</div>
		</div>

		{#if step === 1}
			<ul class="space-y-3 text-sm text-ink-200">
				<li class="flex gap-2.5">
					<Icon name="user" class="mt-0.5 size-4 shrink-0 text-ink-500" />
					<span>Their account is removed and they're signed out everywhere.</span>
				</li>
				<li class="flex gap-2.5">
					<Icon name="trash" class="mt-0.5 size-4 shrink-0 text-primary-300" />
					{#if user.ticketCount > 0}
						<span>
							<strong class="font-semibold text-primary-200">{plural(user.ticketCount, 'ticket')}</strong> they raised
							will be permanently deleted, with all replies and images.
						</span>
					{:else}
						<span>They haven't raised any tickets, so none will be deleted.</span>
					{/if}
				</li>
				{#if user.assignedCount > 0}
					<li class="flex gap-2.5">
						<Icon name="inbox" class="mt-0.5 size-4 shrink-0 text-ink-500" />
						<span>
							{plural(user.assignedCount, 'active ticket')} assigned to them will be unassigned.
						</span>
					</li>
				{/if}
				<li class="flex gap-2.5">
					<Icon name="ticket" class="mt-0.5 size-4 shrink-0 text-ink-500" />
					<span>Replies they posted on other people's tickets stay, shown as “Deleted user”.</span>
				</li>
			</ul>
			<div class="mt-6 flex justify-end gap-2">
				<button type="button" onclick={() => (open = false)} class={ghostBtn}>Cancel</button>
				<button
					type="button"
					onclick={() => (step = 2)}
					class="inline-flex h-9 items-center rounded-lg border border-primary-400/50 px-3.5 text-sm font-medium text-primary-200 transition hover:bg-primary-400/10"
				>
					Continue
				</button>
			</div>
		{:else}
			<form onsubmit={confirmDelete} class="space-y-4">
				{#if error}
					<p role="alert" class="rounded-lg border border-primary-400/25 bg-primary-400/10 px-3 py-2 text-sm text-primary-200">{error}</p>
				{/if}
				<div>
					<label for="confirm-email" class="mb-1.5 block text-[13px] font-medium text-ink-200">
						Type <span class="font-mono text-ink-100 select-all">{user.email}</span> to confirm
					</label>
					<!-- svelte-ignore a11y_autofocus -->
					<input
						id="confirm-email"
						bind:value={typed}
						autocomplete="off"
						spellcheck="false"
						autofocus
						class={['field font-mono text-[13px]', emailMatches && 'border-primary-400/60!']}
					/>
				</div>
				<label class="flex cursor-pointer items-start gap-2.5 text-sm text-ink-200">
					<input type="checkbox" bind:checked={understood} class="mt-0.5 size-4 shrink-0 cursor-pointer accent-[var(--color-primary-600)]" />
					<span>
						I understand
						{user.ticketCount > 0
							? `${plural(user.ticketCount, 'ticket')} and this account will be permanently deleted`
							: 'this account will be permanently deleted'}.
					</span>
				</label>
				<div class="flex justify-between gap-2 pt-2">
					<button type="button" onclick={() => (step = 1)} class={ghostBtn}>Back</button>
					<button type="submit" disabled={!canDelete} class={dangerBtn}>
						<Icon name="trash" class="size-4" />
						{deleting ? 'Deleting…' : 'Delete account'}
					</button>
				</div>
			</form>
		{/if}
	{/if}
</Modal>

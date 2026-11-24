<script lang="ts">
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import {
		admin,
		ApiError,
		type AdminUser,
		type NewUser,
		type ResetRequest,
		type Role
	} from '$lib/api';
	import LithiumSettings from '$lib/components/admin/LithiumSettings.svelte';
	import GroupManager from '$lib/components/admin/GroupManager.svelte';
	import DeleteUserDialog from '$lib/components/admin/DeleteUserDialog.svelte';
	import Dropdown from '$lib/components/Dropdown.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import PasswordField, { passwordsMatch } from '$lib/components/PasswordField.svelte';

	let { data } = $props();

	const tabs = ['users', 'resets', 'groups', 'settings'] as const;
	type Tab = (typeof tabs)[number];
	const tab = $derived<Tab>(tabs.find((t) => t === page.url.searchParams.get('tab')) ?? 'users');

	// --- Notices ---
	let notice = $state<{ kind: 'success' | 'error'; text: string } | null>(null);
	let noticeTimer: ReturnType<typeof setTimeout>;
	function flash(text: string, kind: 'success' | 'error' = 'success') {
		notice = { kind, text };
		clearTimeout(noticeTimer);
		noticeTimer = setTimeout(() => (notice = null), 6000);
	}
	function errorText(err: unknown) {
		if (err instanceof ApiError && err.status < 500) {
			return err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.';
		}
		return 'Something went wrong. Please try again.';
	}

	// --- Users ---
	let query = $state('');
	let users = $state<AdminUser[]>([]);
	let usersLoading = $state(true);
	let usersError = $state('');
	let latestQuery = 0;

	async function loadUsers(q = query) {
		const id = ++latestQuery;
		try {
			const result = await admin.listUsers(q);
			if (id !== latestQuery) return; // a newer search has started
			users = result;
			usersError = '';
		} catch (err) {
			if (id === latestQuery) usersError = errorText(err);
		} finally {
			if (id === latestQuery) usersLoading = false;
		}
	}

	// Debounced search.
	$effect(() => {
		const q = query;
		const t = setTimeout(() => loadUsers(q), q ? 200 : 0);
		return () => clearTimeout(t);
	});

	// --- Reset requests ---
	let resets = $state<ResetRequest[]>([]);
	let resetsLoading = $state(true);
	let resetsError = $state('');

	async function loadResets() {
		try {
			resets = await admin.listResetRequests();
			resetsError = '';
		} catch (err) {
			resetsError = errorText(err);
		} finally {
			resetsLoading = false;
		}
	}
	onMount(loadResets);

	async function dismiss(r: ResetRequest) {
		try {
			await admin.dismissResetRequest(r.id);
			flash(`Dismissed the request from ${r.user.name}.`);
		} catch (err) {
			flash(errorText(err), 'error');
		}
		await Promise.all([loadResets(), loadUsers()]);
	}

	// --- Create user ---
	const emptyUser = (): NewUser => ({ name: '', email: '', role: 'agent', password: '' });
	let createOpen = $state(false);
	let newUser = $state<NewUser>(emptyUser());
	let newUserConfirm = $state('');
	let createError = $state('');
	let creating = $state(false);

	function openCreate() {
		newUser = emptyUser();
		newUserConfirm = '';
		createError = '';
		createOpen = true;
	}

	async function submitCreate(e: SubmitEvent) {
		e.preventDefault();
		createError = '';
		if (!passwordsMatch(newUser.password, newUserConfirm)) {
			createError = "The passwords don't match.";
			return;
		}
		creating = true;
		try {
			const u = await admin.createUser(newUser);
			createOpen = false;
			flash(`Created ${u.name}. Share their password securely. They'll choose their own when they first sign in.`);
			await loadUsers();
		} catch (err) {
			createError = errorText(err);
		} finally {
			creating = false;
		}
	}

	// --- Reset password ---
	let resetOpen = $state(false);
	let resetTarget = $state<{ id: number; name: string; email: string } | null>(null);
	let newPassword = $state('');
	let resetConfirm = $state('');
	let resetError = $state('');
	let resetting = $state(false);
	const isSelf = $derived(resetTarget?.id === data.user?.id);

	function openReset(u: { id: number; name: string; email: string }) {
		resetTarget = u;
		newPassword = '';
		resetConfirm = '';
		resetError = '';
		resetOpen = true;
	}

	async function submitReset(e: SubmitEvent) {
		e.preventDefault();
		if (!resetTarget) return;
		resetError = '';
		if (!passwordsMatch(newPassword, resetConfirm)) {
			resetError = "The passwords don't match.";
			return;
		}
		resetting = true;
		try {
			await admin.setPassword(resetTarget.id, newPassword);
			resetOpen = false;
			flash(
				isSelf
					? 'Your password was changed. Your other sessions were signed out.'
					: `Password updated for ${resetTarget.name}. They've been signed out and will choose a new one when they next sign in.`
			);
			await Promise.all([loadUsers(), loadResets()]);
		} catch (err) {
			resetError = errorText(err);
		} finally {
			resetting = false;
		}
	}

	// --- Formatting ---
	const roleStyles: Record<Role, string> = {
		admin: 'border-primary-400/30 bg-primary-400/10 text-primary-200',
		agent: 'border-ink-700 bg-ink-850 text-ink-200',
		user: 'border-ink-800 bg-transparent text-ink-400'
	};
	const roleLabels: Record<Role, string> = { admin: 'Admin', agent: 'Agent', user: 'User' };
	const roleHints: Record<Role, string> = {
		admin: 'Full access, including this admin area',
		agent: 'Works on tickets, no admin access',
		user: 'Raises and follows their own tickets'
	};
	const roleOptions = (['admin', 'agent', 'user'] as const).map((r) => ({ value: r, label: roleLabels[r] }));
	const isStaffRole = (r: Role) => r === 'admin' || r === 'agent';

	// --- Change role ---
	let roleChange = $state<{ user: AdminUser; role: Role } | null>(null);
	let roleConfirmOpen = $state(false);
	let changingRole = $state(false);

	function requestRole(u: AdminUser, role: Role) {
		// Losing staff access unassigns their tickets, so check first.
		if (isStaffRole(u.role) && role === 'user' && u.assignedCount > 0) {
			roleChange = { user: u, role };
			roleConfirmOpen = true;
		} else {
			applyRole(u, role);
		}
	}

	async function applyRole(u: AdminUser, role: Role) {
		changingRole = true;
		try {
			await admin.setRole(u.id, role);
			flash(`${u.name} is now ${role === 'admin' ? 'an' : 'a'} ${roleLabels[role]}.`);
		} catch (err) {
			flash(errorText(err), 'error');
		} finally {
			changingRole = false;
			roleConfirmOpen = false;
			await loadUsers();
		}
	}

	// --- Reset two-factor ---
	let tfaTarget = $state<AdminUser | null>(null);
	let tfaOpen = $state(false);
	let resettingTfa = $state(false);

	function openResetTfa(u: AdminUser) {
		tfaTarget = u;
		tfaOpen = true;
	}

	async function resetTfa() {
		if (!tfaTarget || resettingTfa) return;
		resettingTfa = true;
		const u = tfaTarget;
		try {
			await admin.resetTwoFactor(u.id);
			flash(
				u.id === data.user?.id
					? 'Your two-factor authentication has been reset.'
					: `Two-factor reset for ${u.name}. They've been signed out and can set it up again when they next sign in.`
			);
		} catch (err) {
			flash(errorText(err), 'error');
		} finally {
			resettingTfa = false;
			tfaOpen = false;
			await loadUsers();
		}
	}

	// --- Delete ---
	let deleteOpen = $state(false);
	let deleteTarget = $state<AdminUser | null>(null);

	function openDelete(u: AdminUser) {
		deleteTarget = u;
		deleteOpen = true;
	}

	async function onDeleted(u: AdminUser, ticketsDeleted: number) {
		flash(
			ticketsDeleted > 0
				? `Deleted ${u.name}'s account and ${ticketsDeleted} ${ticketsDeleted === 1 ? 'ticket' : 'tickets'}.`
				: `Deleted ${u.name}'s account.`
		);
		await Promise.all([loadUsers(), loadResets()]);
	}

	const dateFmt = new Intl.DateTimeFormat(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
	const formatDate = (iso: string) => dateFmt.format(new Date(iso));

	function timeAgo(iso: string) {
		const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
		if (s < 60) return 'just now';
		if (s < 3600) return `${Math.floor(s / 60)}m ago`;
		if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
		if (s < 86400 * 7) return `${Math.floor(s / 86400)}d ago`;
		return formatDate(iso);
	}

	const tabClass = (active: boolean) =>
		[
			'relative flex items-center gap-2 px-1 pb-3 text-sm font-medium transition',
			active
				? 'text-ink-100 after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:rounded-full after:bg-primary-400'
				: 'text-ink-400 hover:text-ink-100'
		].join(' ');

	const secondaryBtn =
		'inline-flex h-8 items-center gap-1.5 rounded-md border border-ink-700 px-2.5 text-[13px] text-ink-200 transition hover:border-primary-400/40 hover:text-primary-200';
	const primaryBtn =
		'inline-flex h-9 items-center justify-center gap-1.5 rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-70';
</script>

<svelte:head>
	<title>Admin · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-7xl px-4 py-10 sm:px-6">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Admin</h1>
			<p class="mt-1 text-sm text-ink-400">Manage who can access your workspace and how Lithium runs.</p>
		</div>
		{#if tab === 'users' || tab === 'resets'}
			<button type="button" onclick={openCreate} class={primaryBtn}>
				<Icon name="plus" class="size-4" /> Create user
			</button>
		{/if}
	</div>

	{#if notice}
		<div
			role="status"
			class={[
				'mt-6 flex items-center gap-2.5 rounded-lg border px-3.5 py-2.5 text-sm',
				notice.kind === 'success'
					? 'border-ink-700 bg-ink-900 text-ink-200'
					: 'border-primary-400/25 bg-primary-400/10 text-primary-200'
			]}
		>
			<Icon name={notice.kind === 'success' ? 'check' : 'close'} class="size-4 shrink-0 text-primary-300" />
			<span class="flex-1">{notice.text}</span>
			<button type="button" onclick={() => (notice = null)} aria-label="Dismiss" class="text-ink-500 hover:text-ink-100">
				<Icon name="close" class="size-4" />
			</button>
		</div>
	{/if}

	<!-- Tabs -->
	<div class="no-scrollbar tab-row-line mt-8 -mx-4 flex gap-6 overflow-x-auto px-4 whitespace-nowrap sm:mx-0 sm:px-0">
		<a href="?tab=users" data-sveltekit-replacestate data-sveltekit-noscroll class={tabClass(tab === 'users')}>
			<Icon name="users" class="size-4" /> Users
		</a>
		<a href="?tab=resets" data-sveltekit-replacestate data-sveltekit-noscroll class={tabClass(tab === 'resets')}>
			<Icon name="key" class="size-4" /> Password resets
			{#if resets.length}
				<span class="rounded-full bg-primary-400 px-1.5 py-px text-[11px] font-bold text-primary-950">
					{resets.length}
				</span>
			{/if}
		</a>
		<a href="?tab=groups" data-sveltekit-replacestate data-sveltekit-noscroll class={tabClass(tab === 'groups')}>
			<Icon name="users" class="size-4" /> Groups
		</a>
		<a href="?tab=settings" data-sveltekit-replacestate data-sveltekit-noscroll class={tabClass(tab === 'settings')}>
			<Icon name="settings" class="size-4" /> Lithium settings
		</a>
	</div>

	{#if tab === 'users'}
		<!-- Search -->
		<div class="relative mt-6 max-w-sm">
			<Icon name="search" class="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-500" />
			<input
				type="search"
				bind:value={query}
				placeholder="Search by name or email"
				aria-label="Search users"
				class="field pl-9"
			/>
		</div>

		<div class="mt-4 rounded-xl border border-ink-800 bg-ink-900/60">
			{#if usersError}
				<p class="px-5 py-10 text-center text-sm text-danger-200">{usersError}</p>
			{:else if usersLoading}
				<p class="px-5 py-10 text-center text-sm text-ink-500">Loading users…</p>
			{:else if users.length === 0}
				<div class="px-5 py-14 text-center">
					<p class="text-sm font-medium text-ink-200">No users found</p>
					<p class="mt-1 text-sm text-ink-500">
						{query ? `Nothing matches "${query}".` : 'Create the first user to get started.'}
					</p>
				</div>
			{:else}
				<table class="w-full text-left text-sm">
					<thead class="border-b border-ink-800 text-xs text-ink-500">
						<tr>
							<th class="px-5 py-3 font-medium">User</th>
							<th class="hidden px-5 py-3 font-medium sm:table-cell">Role</th>
							<th class="hidden px-5 py-3 font-medium md:table-cell">Created</th>
							<th class="px-5 py-3"><span class="sr-only">Actions</span></th>
						</tr>
					</thead>
					<tbody class="divide-y divide-ink-800">
						{#each users as u (u.id)}
							<tr class="transition hover:bg-ink-850/50">
								<td class="px-5 py-3">
									<div class="flex items-center gap-3">
										<Avatar name={u.name} src={u.avatarUrl} size={32} />
										<div class="min-w-0">
											<p class="flex items-center gap-2 truncate font-medium text-ink-100">
												{u.name}
												{#if u.id === data.user?.id}
													<span class="text-xs font-normal text-ink-500">(you)</span>
												{/if}
												{#if u.mustChangePassword && !u.pendingReset}
													<span
														title="Still using a password set by an admin"
														class="rounded-full border border-ink-700 px-1.5 py-px text-[11px] font-normal text-ink-400"
													>
														Temporary password
													</span>
												{/if}
												{#if u.twoFactorEnabled}
													<span
														title="Uses two-factor authentication"
														class="inline-flex items-center gap-0.5 rounded-full border border-ink-700 px-1.5 py-px text-[11px] font-normal text-ink-400"
													>
														<Icon name="shield" class="size-3" /> 2FA
													</span>
												{/if}
												{#if u.pendingReset}
													<a
														href="?tab=resets"
														data-sveltekit-replacestate
														class="rounded-full border border-primary-400/30 bg-primary-400/10 px-1.5 py-px text-[11px] font-medium text-primary-200"
													>
														Reset requested
													</a>
												{/if}
											</p>
											<p class="truncate text-ink-400">{u.email}</p>
										</div>
									</div>
								</td>
								<td class="hidden px-5 py-3 sm:table-cell">
									{#if u.id === data.user?.id}
										<!-- You can't change your own role, so there's always an admin. -->
										<span
											title="You can't change your own role"
											class={['inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-medium', roleStyles[u.role]]}
										>
											{roleLabels[u.role]}
										</span>
									{:else}
										<div class="w-32">
											{#key u.role}
												<Dropdown
													id="role-{u.id}"
													size="sm"
													menu="wide"
													label="Role for {u.name}"
													value={u.role}
													options={roleOptions}
													disabled={changingRole}
													onchange={(r) => requestRole(u, r)}
												>
													{#snippet item(opt)}
														<span class="min-w-0">
															<span class={['inline-block rounded-md border px-2 py-0.5 text-xs font-medium', roleStyles[opt.value]]}>
																{opt.label}
															</span>
															<span class="mt-1 block text-xs text-ink-500">{roleHints[opt.value]}</span>
														</span>
													{/snippet}
													{#snippet selected(opt)}
														<span class={['rounded-md border px-2 py-0.5 text-xs font-medium', roleStyles[opt.value]]}>{opt.label}</span>
													{/snippet}
												</Dropdown>
											{/key}
										</div>
									{/if}
								</td>
								<td class="hidden px-5 py-3 text-ink-400 md:table-cell">{formatDate(u.createdAt)}</td>
								<td class="px-5 py-3 text-right">
									<div class="flex items-center justify-end gap-1.5">
										{#if u.twoFactorEnabled}
											<button
												type="button"
												onclick={() => openResetTfa(u)}
												aria-label="Reset two-factor for {u.name}"
												title="Reset two-factor"
												class="flex size-8 items-center justify-center rounded-md border border-ink-700 text-ink-400 transition hover:border-ink-600 hover:bg-ink-850 hover:text-ink-100"
											>
												<Icon name="phone" class="size-3.5" />
											</button>
										{:else}
											<span class="size-8" aria-hidden="true"></span>
										{/if}
										<button type="button" onclick={() => openReset(u)} class={secondaryBtn}>
											<Icon name="key" class="size-3.5" />
											<span class="hidden sm:inline">Reset password</span>
											<span class="sm:hidden">Reset</span>
										</button>
										{#if u.id !== data.user?.id}
											<button
												type="button"
												onclick={() => openDelete(u)}
												aria-label="Delete {u.name}"
												title="Delete account"
												class="flex size-8 items-center justify-center rounded-md border border-ink-700 text-ink-400 transition hover:border-primary-400/50 hover:bg-primary-400/10 hover:text-primary-300"
											>
												<Icon name="trash" class="size-3.5" />
											</button>
										{:else}
											<span class="size-8" aria-hidden="true"></span>
										{/if}
									</div>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
		{#if !usersLoading && users.length}
			<p class="mt-3 text-xs text-ink-500">{users.length} {users.length === 1 ? 'user' : 'users'}</p>
		{/if}
	{:else if tab === 'resets'}
		<p class="mt-6 max-w-2xl text-sm text-ink-400">
			Users who forgot their password asked for help here. Check it's really them, then set a new
			password and share it securely.
		</p>

		<div class="mt-4 overflow-hidden rounded-xl border border-ink-800 bg-ink-900/60">
			{#if resetsError}
				<p class="px-5 py-10 text-center text-sm text-danger-200">{resetsError}</p>
			{:else if resetsLoading}
				<p class="px-5 py-10 text-center text-sm text-ink-500">Loading requests…</p>
			{:else if resets.length === 0}
				<div class="flex flex-col items-center px-5 py-14 text-center">
					<span class="flex size-11 items-center justify-center rounded-xl bg-primary-400/10 text-primary-400">
						<Icon name="check" class="size-5" />
					</span>
					<p class="mt-4 text-sm font-medium text-ink-200">All caught up</p>
					<p class="mt-1 text-sm text-ink-500">No pending password reset requests.</p>
				</div>
			{:else}
				<ul class="divide-y divide-ink-800">
					{#each resets as r (r.id)}
						<li class="flex flex-wrap items-center gap-x-4 gap-y-3 px-5 py-4">
							<div class="min-w-0 flex-1">
								<p class="truncate font-medium text-ink-100">{r.user.name}</p>
								<p class="truncate text-sm text-ink-400">{r.user.email}</p>
							</div>
							<div class="text-right text-xs text-ink-500">
								<p title={new Date(r.createdAt).toLocaleString()}>Requested {timeAgo(r.createdAt)}</p>
								{#if r.ip}<p class="font-mono">{r.ip}</p>{/if}
							</div>
							<div class="flex gap-2">
								<button type="button" onclick={() => dismiss(r)} class={secondaryBtn}>Dismiss</button>
								<button
									type="button"
									onclick={() => openReset(r.user)}
									class="inline-flex h-8 items-center gap-1.5 rounded-md bg-primary-400 px-2.5 text-[13px] font-semibold text-primary-950 transition hover:bg-primary-300"
								>
									<Icon name="key" class="size-3.5" /> Set new password
								</button>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{:else if tab === 'groups'}
		<GroupManager />
	{:else}
		<LithiumSettings />
	{/if}
</div>

<!-- Create user -->
<Modal bind:open={createOpen} title="Create user" description="They can sign in straight away with this password.">
	<form onsubmit={submitCreate} class="space-y-4">
		{#if createError}
			<p role="alert" class="rounded-lg border border-danger-400/25 bg-danger-400/10 px-3 py-2 text-sm text-danger-200">
				{createError}
			</p>
		{/if}
		<div>
			<label for="new-name" class="mb-1.5 block text-[13px] font-medium text-ink-200">Full name</label>
			<input id="new-name" required maxlength="100" bind:value={newUser.name} placeholder="Alex Morgan" class="field" />
		</div>
		<div>
			<label for="new-email" class="mb-1.5 block text-[13px] font-medium text-ink-200">Email</label>
			<input
				id="new-email"
				type="email"
				required
				autocomplete="off"
				bind:value={newUser.email}
				placeholder="alex@company.com"
				class="field"
			/>
		</div>
		<fieldset>
			<legend class="mb-1.5 block text-[13px] font-medium text-ink-200">Role</legend>
			<div class="grid grid-cols-3 gap-2">
				{#each ['agent', 'admin', 'user'] as const as role (role)}
					<label
						class={[
							'flex cursor-pointer flex-col rounded-lg border px-3 py-2 text-sm transition',
							newUser.role === role
								? 'border-primary-400/60 bg-primary-400/10 text-ink-100'
								: 'border-ink-700 text-ink-400 hover:border-ink-500'
						]}
					>
						<input type="radio" name="role" value={role} bind:group={newUser.role} class="sr-only" />
						<span class="font-medium">{roleLabels[role]}</span>
					</label>
				{/each}
			</div>
			<p class="mt-1.5 text-xs text-ink-500">
				{#if newUser.role === 'admin'}
					Full access, including this admin area.
				{:else if newUser.role === 'agent'}
					Works on tickets. No admin access.
				{:else}
					Can raise and follow their own tickets.
				{/if}
			</p>
		</fieldset>
		<PasswordField id="new-password" confirm generator bind:value={newUser.password} bind:confirmValue={newUserConfirm} />
		<p class="-mt-2 text-xs text-ink-500">They'll be asked to choose their own password when they first sign in.</p>
		<div class="flex justify-end gap-2 pt-2">
			<button type="button" onclick={() => (createOpen = false)} class="h-9 rounded-lg px-3.5 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100">
				Cancel
			</button>
			<button type="submit" disabled={creating} class={primaryBtn}>
				{creating ? 'Creating…' : 'Create user'}
			</button>
		</div>
	</form>
</Modal>

<!-- Reset password -->
<Modal
	bind:open={resetOpen}
	title={isSelf ? 'Change your password' : 'Reset password'}
	description={resetTarget
		? isSelf
			? 'Your other sessions will be signed out. You stay signed in here.'
			: `Set a temporary password for ${resetTarget.name} (${resetTarget.email}). They'll be signed out everywhere and must choose a new one when they next sign in.`
		: undefined}
>
	<form onsubmit={submitReset} class="space-y-4">
		{#if resetError}
			<p role="alert" class="rounded-lg border border-danger-400/25 bg-danger-400/10 px-3 py-2 text-sm text-danger-200">
				{resetError}
			</p>
		{/if}
		<PasswordField
			id="reset-password"
			label="New password"
			confirm
			generator={!isSelf}
			bind:value={newPassword}
			bind:confirmValue={resetConfirm}
		/>
		<div class="flex justify-end gap-2 pt-2">
			<button type="button" onclick={() => (resetOpen = false)} class="h-9 rounded-lg px-3.5 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100">
				Cancel
			</button>
			<button type="submit" disabled={resetting} class={primaryBtn}>
				{resetting ? 'Saving…' : 'Set password'}
			</button>
		</div>
	</form>
</Modal>

<!-- Change role (only asked when demoting staff who have tickets assigned) -->
<Modal
	bind:open={roleConfirmOpen}
	title={roleChange ? `Change ${roleChange.user.name} to ${roleLabels[roleChange.role]}?` : 'Change role?'}
	description="They'll lose staff access straight away."
>
	{#if roleChange}
		<p class="text-sm text-ink-200">
			They're assigned {roleChange.user.assignedCount}
			active {roleChange.user.assignedCount === 1 ? 'ticket' : 'tickets'}. Only staff can be assigned, so
			{roleChange.user.assignedCount === 1 ? 'it' : 'they'} will be unassigned and go back to the queue.
		</p>
		<div class="mt-6 flex justify-end gap-2">
			<button type="button" onclick={() => (roleConfirmOpen = false)} class="h-9 rounded-lg px-3.5 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100">
				Cancel
			</button>
			<button
				type="button"
				disabled={changingRole}
				onclick={() => roleChange && applyRole(roleChange.user, roleChange.role)}
				class={primaryBtn}
			>
				{changingRole ? 'Changing…' : 'Change role'}
			</button>
		</div>
	{/if}
</Modal>

<!-- Reset two-factor -->
<Modal
	bind:open={tfaOpen}
	title={tfaTarget ? `Reset two-factor for ${tfaTarget.name}?` : 'Reset two-factor?'}
	description="Use this when someone has lost their phone and recovery codes."
>
	{#if tfaTarget}
		<p class="text-sm text-ink-200">
			{#if tfaTarget.id === data.user?.id}
				This removes your authenticator app and recovery codes. Your other devices will be signed out.
			{:else}
				This removes {tfaTarget.name}'s authenticator app and recovery codes and signs them out everywhere.
				Make sure it's really them asking.
			{/if}
			{#if data.settings.requireTwoFactor}
				Two-factor is mandatory, so {tfaTarget.id === data.user?.id ? "you'll" : "they'll"} set it up again
				{tfaTarget.id === data.user?.id ? 'straight away' : 'when they next sign in'}.
			{/if}
		</p>
		<div class="mt-6 flex justify-end gap-2">
			<button type="button" onclick={() => (tfaOpen = false)} class="h-9 rounded-lg px-3.5 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100">
				Cancel
			</button>
			<button type="button" disabled={resettingTfa} onclick={resetTfa} class={primaryBtn}>
				{resettingTfa ? 'Resetting…' : 'Reset two-factor'}
			</button>
		</div>
	{/if}
</Modal>

<DeleteUserDialog bind:open={deleteOpen} user={deleteTarget} ondeleted={onDeleted} />

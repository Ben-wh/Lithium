<script lang="ts">
	import { afterNavigate, goto } from '$app/navigation';
	import { page } from '$app/state';
	import { logout, type User } from '$lib/api';
	import Icon, { type IconName } from './Icon.svelte';
	import BrandLogo from './BrandLogo.svelte';
	import Avatar from './Avatar.svelte';
	import ThemeSwitcher from './ThemeSwitcher.svelte';
	import NotificationBell from './NotificationBell.svelte';

	let { user }: { user: User } = $props();

	// An uploaded logo without a company name is shown on its own (it's
	// probably a wordmark); otherwise the name sits beside the logo.
	const companyName = $derived(page.data.branding?.companyName ?? '');
	const brandName = $derived(companyName || (page.data.branding?.logoUrl ? '' : 'Lithium'));

	const allLinks: { href: string; label: string; icon: IconName; staffOnly?: boolean }[] = [
		{ href: '/', label: 'Dashboard', icon: 'home' },
		{ href: '/tickets', label: 'Tickets', icon: 'ticket' },
		{ href: '/user-book', label: 'User book', icon: 'users', staffOnly: true },
		{ href: '/knowledge-base', label: 'Knowledge base', icon: 'book' },
		{ href: '/reports', label: 'Reports', icon: 'chart', staffOnly: true }
	];
	const links = $derived(allLinks.filter((l) => !l.staffOnly || user.role === 'admin' || user.role === 'agent'));

	const isAdmin = $derived(user.role === 'admin');

	function isActive(href: string) {
		const p = page.url.pathname;
		return href === '/' ? p === '/' : p === href || p.startsWith(href + '/');
	}

	const menuItem =
		'flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm text-ink-200 transition hover:bg-ink-850 hover:text-ink-100 focus-visible:bg-ink-850 focus-visible:outline-none';

	let profileOpen = $state(false);
	let mobileOpen = $state(false);
	let profileEl: HTMLElement | undefined = $state();

	afterNavigate(() => {
		profileOpen = false;
		mobileOpen = false;
	});

	function onWindowClick(e: MouseEvent) {
		if (profileOpen && profileEl && !profileEl.contains(e.target as Node)) profileOpen = false;
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			profileOpen = false;
			mobileOpen = false;
		}
	}

	async function signOut() {
		await logout();
		await goto('/login', { invalidateAll: true });
	}
</script>

<svelte:window onclick={onWindowClick} onkeydown={onKeydown} />

<header class="sticky top-0 z-40 border-b border-ink-800 bg-ink-950/80 backdrop-blur-md">
	<div class="mx-auto flex h-14 max-w-7xl items-center gap-6 px-4 sm:px-6">
		<!-- Brand -->
		<a
			href="/"
			aria-label={brandName ? undefined : 'Home'}
			class="flex min-w-0 shrink-0 items-center gap-2.5 rounded-md focus-visible:ring-2 focus-visible:ring-primary-400/40 focus-visible:outline-none"
		>
			<BrandLogo size={26} maxWidth={160} />
			{#if brandName}
				<span class="max-w-48 truncate text-[15px] font-semibold tracking-tight text-ink-100">{brandName}</span>
			{/if}
		</a>

		<!-- Primary nav (desktop) -->
		<nav class="hidden items-center gap-0.5 md:flex" aria-label="Main">
			{#each links as link (link.href)}
				{@const active = isActive(link.href)}
				<a
					href={link.href}
					aria-current={active ? 'page' : undefined}
					class={[
						'relative flex items-center gap-2 rounded-md px-3 py-1.5 text-sm transition',
						'focus-visible:ring-2 focus-visible:ring-primary-400/40 focus-visible:outline-none',
						active ? 'bg-ink-850 text-ink-100' : 'text-ink-400 hover:bg-ink-900 hover:text-ink-100'
					]}
				>
					<Icon name={link.icon} class={['size-4', active ? 'text-primary-400' : ''].join(' ')} />
					{link.label}
				</a>
			{/each}
		</nav>

		<div class="ml-auto flex items-center gap-2">
			<a
				href="/tickets/new"
				class="hidden h-8 items-center gap-1.5 rounded-md bg-primary-400 px-3 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 focus-visible:ring-2 focus-visible:ring-primary-400/40 focus-visible:ring-offset-2 focus-visible:ring-offset-ink-950 focus-visible:outline-none sm:flex"
			>
				<Icon name="plus" class="size-4" />
				New ticket
			</a>

			{#if isAdmin}
				<a
					href="/admin"
					aria-current={isActive('/admin') ? 'page' : undefined}
					class={[
						'hidden h-8 items-center gap-1.5 rounded-md border px-3 text-sm transition sm:flex',
						'focus-visible:ring-2 focus-visible:ring-primary-400/40 focus-visible:outline-none',
						isActive('/admin')
							? 'border-primary-400/50 bg-primary-400/10 text-primary-200'
							: 'border-ink-700 text-ink-200 hover:border-primary-400/40 hover:text-primary-200'
					]}
				>
					<Icon name="shield" class="size-4" />
					Admin
				</a>
			{/if}

			<NotificationBell />

			<!-- Profile -->
			<div class="relative" bind:this={profileEl}>
				<button
					type="button"
					onclick={() => (profileOpen = !profileOpen)}
					aria-haspopup="menu"
					aria-expanded={profileOpen}
					aria-label="Account menu"
					class="flex items-center gap-1 rounded-full p-0.5 pr-1.5 transition hover:bg-ink-900 focus-visible:ring-2 focus-visible:ring-primary-400/40 focus-visible:outline-none"
				>
					<Avatar name={user.name} src={user.avatarUrl} size={28} tone="primary" />
					<Icon name="chevron" class={['size-3.5 text-ink-500 transition', profileOpen ? 'rotate-180' : ''].join(' ')} />
				</button>

				{#if profileOpen}
					<div
						role="menu"
						class="absolute right-0 mt-2 w-64 origin-top-right rounded-xl border border-ink-800 bg-ink-900 p-1.5 shadow-2xl shadow-shade/50"
					>
						<div class="flex items-center gap-3 px-2.5 py-2.5">
							<Avatar name={user.name} src={user.avatarUrl} size={36} tone="primary" />
							<div class="min-w-0">
								<p class="truncate text-sm font-medium text-ink-100">{user.name}</p>
								<p class="truncate text-xs text-ink-400">{user.email}</p>
							</div>
						</div>
						<div class="my-1 h-px bg-ink-800"></div>
						<a href="/profile" role="menuitem" class={menuItem}>
							<Icon name="user" class="size-4 text-ink-400" /> Your profile
						</a>
						{#if isAdmin}
							<a href="/admin" role="menuitem" class="{menuItem} sm:hidden">
								<Icon name="shield" class="size-4 text-ink-400" /> Admin
							</a>
						{/if}
						<div class="my-1 h-px bg-ink-800"></div>
						<div class="px-2.5 py-2">
							<p class="mb-1.5 text-xs text-ink-500">Theme</p>
							<ThemeSwitcher showLabels />
						</div>
						<div class="my-1 h-px bg-ink-800"></div>
						<button type="button" role="menuitem" onclick={signOut} class="{menuItem} w-full hover:text-primary-300!">
							<Icon name="logout" class="size-4 text-ink-400" /> Sign out
						</button>
					</div>
				{/if}
			</div>

			<!-- Mobile menu toggle -->
			<button
				type="button"
				onclick={() => (mobileOpen = !mobileOpen)}
				aria-expanded={mobileOpen}
				aria-controls="mobile-nav"
				aria-label={mobileOpen ? 'Close menu' : 'Open menu'}
				class="flex size-8 items-center justify-center rounded-md text-ink-400 transition hover:bg-ink-900 hover:text-ink-100 md:hidden"
			>
				<Icon name={mobileOpen ? 'close' : 'menu'} class="size-5" />
			</button>
		</div>
	</div>

	<!-- Mobile nav -->
	{#if mobileOpen}
		<nav id="mobile-nav" class="border-t border-ink-800 px-4 py-3 md:hidden" aria-label="Main">
			<div class="flex flex-col gap-0.5">
				{#each links as link (link.href)}
					{@const active = isActive(link.href)}
					<a
						href={link.href}
						aria-current={active ? 'page' : undefined}
						class={[
							'flex items-center gap-3 rounded-md px-3 py-2 text-sm',
							active ? 'bg-ink-850 text-ink-100' : 'text-ink-400 hover:bg-ink-900 hover:text-ink-100'
						]}
					>
						<Icon name={link.icon} class={['size-4', active ? 'text-primary-400' : ''].join(' ')} />
						{link.label}
					</a>
				{/each}
			</div>
			<a
				href="/tickets/new"
				class="mt-3 flex h-9 items-center justify-center gap-1.5 rounded-md bg-primary-400 text-sm font-semibold text-primary-950 sm:hidden"
			>
				<Icon name="plus" class="size-4" /> New ticket
			</a>
		</nav>
	{/if}
</header>


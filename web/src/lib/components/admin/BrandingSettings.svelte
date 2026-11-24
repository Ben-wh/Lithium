<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { page } from '$app/state';
	import { adminBranding, adminSettings, ApiError, COMPANY_NAME_MAX, IMAGE_LIMITS, MAX_LOGO_BYTES } from '$lib/api';
	import { formatBytes } from '$lib/format';
	import Icon from '$lib/components/Icon.svelte';
	import Logo from '$lib/components/Logo.svelte';

	// Company name and logo, shown in the navbar and on the sign-in pages.
	const savedName = $derived(page.data.settings?.companyName ?? '');
	const logoUrl = $derived(page.data.settings?.logoUrl ?? '');
	let name = $state(page.data.settings?.companyName ?? '');
	const nameChanged = $derived(name.trim().replace(/\s+/g, ' ') !== savedName);

	let saving = $state(false);
	let status = $state<'saved' | 'error' | null>(null);
	let errorText = $state('');
	let statusTimer: ReturnType<typeof setTimeout>;
	let fileInput: HTMLInputElement | undefined = $state();

	// What the navbar shows: an uploaded logo with no name stands on its own.
	const previewName = $derived(name.trim() || (logoUrl ? '' : 'Lithium'));

	async function run(action: () => Promise<unknown>) {
		saving = true;
		clearTimeout(statusTimer);
		try {
			await action();
			await invalidate('app:settings'); // the navbar and sign-in pages pick it up straight away
			name = page.data.settings?.companyName ?? name;
			status = 'saved';
		} catch (e) {
			errorText = e instanceof ApiError ? e.message : "Couldn't save. Try again.";
			status = 'error';
		} finally {
			saving = false;
			statusTimer = setTimeout(() => (status = null), status === 'saved' ? 2000 : 5000);
		}
	}

	function onPick(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = ''; // let the same file be picked again
		if (!file) return;
		if (!(IMAGE_LIMITS.types as readonly string[]).includes(file.type)) {
			errorText = "That isn't a PNG, JPEG, GIF or WebP image.";
			status = 'error';
			return;
		}
		if (file.size > MAX_LOGO_BYTES) {
			errorText = `Logos must be ${formatBytes(MAX_LOGO_BYTES)} or smaller.`;
			status = 'error';
			return;
		}
		run(() => adminBranding.uploadLogo(file));
	}
</script>

<div class="mt-5 rounded-xl border border-ink-800 bg-ink-900/60">
	<div class="flex items-center justify-between gap-3 border-b border-ink-800 px-5 py-4">
		<div>
			<h3 class="text-sm font-semibold">Company branding</h3>
			<p class="mt-0.5 text-sm text-ink-400">Shown in the navbar and on the sign-in pages, for everyone.</p>
		</div>
		<span class="shrink-0 text-xs" aria-live="polite">
			{#if saving}
				<span class="text-ink-500">Saving…</span>
			{:else if status === 'saved'}
				<span class="flex items-center gap-1 text-ink-400"><Icon name="check" class="size-3.5 text-primary-300" /> Saved</span>
			{:else if status === 'error'}
				<span class="text-danger-300">{errorText}</span>
			{/if}
		</span>
	</div>

	<div class="space-y-6 px-5 py-4">
		<!-- Logo -->
		<div>
			<span class="mb-1.5 block text-[13px] font-medium text-ink-200">Logo</span>
			<div class="flex flex-wrap items-center gap-4">
				<div class="flex h-20 w-44 items-center justify-center rounded-lg border border-ink-800 bg-ink-950 p-3">
					{#if logoUrl}
						<img src={logoUrl} alt="Current company logo" class="max-h-full max-w-full object-contain" />
					{:else}
						<Logo size={44} />
					{/if}
				</div>
				<div class="flex flex-wrap gap-2">
					<input
						bind:this={fileInput}
						type="file"
						accept={IMAGE_LIMITS.types.join(',')}
						class="hidden"
						onchange={onPick}
					/>
					<button
						type="button"
						disabled={saving}
						onclick={() => fileInput?.click()}
						class="flex h-9 items-center gap-2 rounded-lg bg-primary-400 px-3 text-[13px] font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-40"
					>
						<Icon name="upload" class="size-4" />
						{logoUrl ? 'Replace logo' : 'Upload logo'}
					</button>
					{#if logoUrl}
						<button
							type="button"
							disabled={saving}
							onclick={() => run(() => adminBranding.removeLogo())}
							class="flex h-9 items-center gap-2 rounded-lg border border-ink-700 px-3 text-[13px] text-ink-300 transition hover:border-ink-500 hover:text-ink-100 disabled:cursor-not-allowed disabled:opacity-40"
						>
							<Icon name="trash" class="size-4" />
							Remove
						</button>
					{/if}
				</div>
			</div>
			<p class="mt-1.5 text-xs text-ink-500">
				PNG, JPEG, GIF or WebP, up to {formatBytes(MAX_LOGO_BYTES)}. A transparent background works best in both
				light and dark themes. Without one, the Lithium logo is used.
			</p>
		</div>

		<!-- Company name -->
		<form
			onsubmit={(e) => {
				e.preventDefault();
				run(() => adminSettings.update({ companyName: name }));
			}}
		>
			<label for="company-name" class="mb-1.5 block text-[13px] font-medium text-ink-200">Company name</label>
			<div class="flex flex-wrap gap-2">
				<input
					id="company-name"
					bind:value={name}
					maxlength={COMPANY_NAME_MAX}
					placeholder="Lithium"
					class="field h-9 min-w-0 flex-1 text-[13px]"
				/>
				<button
					type="submit"
					disabled={saving || !nameChanged}
					class="h-9 rounded-lg bg-primary-400 px-3 text-[13px] font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-40"
				>
					Save
				</button>
			</div>
			<p class="mt-1.5 text-xs text-ink-500">
				Up to {COMPANY_NAME_MAX} characters. Leave blank to show just your logo, or "Lithium" if there's no logo.
			</p>
		</form>

		<!-- Preview -->
		<div>
			<p class="mb-1.5 text-xs text-ink-500">Navbar preview</p>
			<div class="flex h-14 items-center gap-2.5 rounded-lg border border-ink-800 bg-ink-950/80 px-4">
				{#if logoUrl}
					<img src={logoUrl} alt="" class="h-6.5 max-w-40 w-auto object-contain" />
				{:else}
					<Logo size={26} />
				{/if}
				{#if previewName}
					<span class="max-w-48 truncate text-[15px] font-semibold tracking-tight text-ink-100">{previewName}</span>
				{/if}
			</div>
		</div>
	</div>
</div>

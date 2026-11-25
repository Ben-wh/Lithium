import { redirect } from '@sveltejs/kit';
import { ApiError, me, publicBanner, publicBranding, type User } from '$lib/api';
import type { LayoutLoad } from './$types';

export const ssr = false;
export const prerender = false;

const publicRoutes = ['/login', '/forgot-password'];
const SETUP_2FA = '/two-factor-setup';

export const load: LayoutLoad = async ({ fetch, url, depends }) => {
	depends('app:settings'); // refresh the banner and branding when an admin changes them
	let user: User | null = null;
	const [meResult, banner, branding] = await Promise.all([
		me(fetch).catch((e) => e),
		// The sensitive mode banner and company branding show on the sign-in pages too.
		publicBanner(fetch).catch(() => ({ sensitiveMode: false, sensitiveMessage: '', lockdown: false })),
		publicBranding(fetch).catch(() => ({ companyName: '', logoUrl: '', accent: 'coral' as const }))
	]);
	if (meResult instanceof Error) {
		if (!(meResult instanceof ApiError && meResult.status === 401)) throw meResult;
	} else {
		user = meResult;
	}

	const isPublic = publicRoutes.includes(url.pathname);
	if (!user) {
		if (!isPublic) {
			const next = url.pathname === '/' ? '' : `?next=${encodeURIComponent(url.pathname + url.search)}`;
			redirect(307, `/login${next}`);
		}
		return { user, banner, branding };
	}

	// Signing in goes: password → two-factor code (on the login page) →
	// choose own password if an admin set it → set up 2FA if it's mandatory.
	if (user.twoFactor === 'verify') {
		if (url.pathname !== '/login') redirect(307, '/login');
		return { user, banner, branding };
	}
	const onChangePage = url.pathname === '/change-password';
	if (user.mustChangePassword) {
		if (!onChangePage) redirect(307, '/change-password');
		return { user, banner, branding };
	}
	const onSetupPage = url.pathname === SETUP_2FA;
	if (user.twoFactor === 'setup') {
		if (!onSetupPage) redirect(307, SETUP_2FA);
		return { user, banner, branding };
	}
	if (onChangePage || onSetupPage || isPublic) redirect(307, '/');

	return { user, banner, branding };
};

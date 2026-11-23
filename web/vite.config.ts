import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		// During development, run the Go server on :8080 and Vite proxies the API.
		proxy: { '/api': 'http://localhost:8080' }
	}
});

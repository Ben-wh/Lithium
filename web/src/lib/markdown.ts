import DOMPurify from 'dompurify';
import { marked } from 'marked';

// Knowledge base articles are written in Markdown and rendered to HTML in
// the browser. The output is always sanitised, so an article can't inject
// scripts or styles.

marked.setOptions({ gfm: true, breaks: true });

// Links to other sites open in a new tab.
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
	if (node.tagName === 'A') {
		const href = node.getAttribute('href') ?? '';
		if (/^https?:\/\//i.test(href)) {
			node.setAttribute('target', '_blank');
			node.setAttribute('rel', 'noopener noreferrer');
		}
	}
});

export function renderMarkdown(md: string): string {
	const html = marked.parse(md, { async: false });
	return DOMPurify.sanitize(html, { USE_PROFILES: { html: true } });
}

// Two languages, Vietnamese and English.
//
// - Text built in JavaScript: t('tiếng Việt', 'English'), the pair written where it is used.
// - Static text in index.html stays Vietnamese in the markup; its English lives in
//   i18n-html.js, keyed by the Vietnamese source. The page is scanned once at start
//   (before anything is rendered into it) and each piece of static text is swapped
//   when the language changes.
import EN from './i18n-html.js';

let lang = 'vi';

export const getLang = () => lang;

/** The text in the current language. */
export const t = (vi, en) => (lang === 'en' ? en : vi);

// Inline formatting that may sit inside a sentence: such a sentence is translated as a whole.
const INLINE = new Set(['B', 'I', 'EM', 'STRONG', 'CODE', 'SMALL', 'SPAN', 'BR', 'A', 'KBD']);
const ATTRS = ['placeholder', 'title', 'aria-label', 'data-linux-placeholder'];
const SKIP = new Set(['SCRIPT', 'STYLE', 'PRE', 'TEXTAREA']);

const norm = (s) => s.replace(/\s+/g, ' ').trim();
const hasLetters = (s) => /\p{L}/u.test(s);

/** @type {{apply: (en: boolean) => void, key: string}[]} */
const units = [];

function inlineOnly(el) {
    for (const d of el.querySelectorAll('*')) {
        if (!INLINE.has(d.tagName) || d.id) return false;
    }
    return true;
}

function scan(root) {
    const walk = (el) => {
        for (const a of ATTRS) {
            const v = el.getAttribute(a);
            if (v && hasLetters(v)) {
                units.push({key: norm(v), apply: (en) => el.setAttribute(a, en && EN[norm(v)] || v)});
            }
        }
        if (SKIP.has(el.tagName)) return; // attributes yes, contents no
        const texts = [...el.childNodes].filter((n) => n.nodeType === Node.TEXT_NODE && hasLetters(n.nodeValue));
        if (texts.length && inlineOnly(el)) {
            // A whole sentence, possibly with <b>/<code>… inside: one unit.
            const html = el.innerHTML;
            const key = norm(html);
            units.push({key, apply: (en) => { el.innerHTML = en && EN[key] || html; }});
            for (const d of el.querySelectorAll('*')) {
                for (const a of ATTRS) {
                    const v = d.getAttribute(a);
                    if (v && hasLetters(v)) units.push({key: norm(v), apply: (en) => d.setAttribute(a, en && EN[norm(v)] || v)});
                }
            }
            return;
        }
        // Text next to interactive children (buttons, inputs, elements with ids): keep
        // the structure, translate each text node on its own.
        for (const n of texts) {
            const raw = n.nodeValue;
            const key = norm(raw);
            const lead = raw.match(/^\s*/)[0];
            const tail = raw.match(/\s*$/)[0];
            units.push({key, apply: (en) => { n.nodeValue = en && EN[key] ? lead + EN[key] + tail : raw; }});
        }
        for (const c of el.children) walk(c);
    };
    walk(root);
}

/** Records the static text of the page. Call once, before rendering into it. */
export function initI18n() {
    scan(document.body);
    if (document.title) units.push({key: document.title, apply: () => {}});
}

/** Switches language: 'vi' or 'en'. Returns whether it changed. */
export function setLang(next) {
    next = next === 'en' ? 'en' : 'vi';
    if (next === lang) return false;
    lang = next;
    document.documentElement.lang = lang;
    for (const u of units) u.apply(lang === 'en');
    return true;
}

/** Static texts that have no English yet (for development: run in the console). */
export function missingTranslations() {
    // Pure ASCII text (URLs, "example.com", "steam.exe") is the same in both languages.
    return [...new Set(units.map((u) => u.key))].filter((k) => !(k in EN) && /[^\x00-\x7F]/.test(k));
}

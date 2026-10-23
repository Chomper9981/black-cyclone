import './style.css';
import './app.css';

import {
    GetSettings,
    SaveSettings,
    RegisterWarp,
    Connect,
    Disconnect,
    GetStatus,
} from '../wailsjs/go/main/App';
import {EventsOn} from '../wailsjs/runtime/runtime';

const $ = (id) => document.getElementById(id);

const el = {
    pill: $('pill'),
    warnings: $('warnings'),
    toggle: $('toggle'),
    toggleLabel: $('toggle-label'),
    duration: $('duration'),
    error: $('error'),
    modes: $('modes'),
    modeInputs: [...document.querySelectorAll('input[name="mode"]')],
    warpPanel: $('warp-panel'),
    warpNone: $('warp-none'),
    warpHas: $('warp-has'),
    warpRegister: $('warp-register'),
    warpReregister: $('warp-reregister'),
    warpMsg: $('warp-msg'),
    customPanel: $('custom-panel'),
    customLink: $('custom-link'),
    doh: $('doh'),
    dpi: $('dpi'),
    dpiRow: $('dpi-row'),
    lan: $('lan'),
    saveMsg: $('save-msg'),
    log: $('log'),
    logCount: $('log-count'),
    logClear: $('log-clear'),
};

const LOG_MAX = 500;
const SAVE_DELAY = 500;
const POLL_MS = 3000;

let settings = null;          // last known Settings (local edits applied)
let status = {running: false, mode: '', since: '', admin: true, bins_ok: true, missing: [], error: ''};
let busy = false;             // Connect/Disconnect in flight
let warpBusy = false;         // RegisterWarp in flight
let callError = '';           // error from last rejected call
let saveTimer = null;
let savePromise = null;
let reregisterArmed = null;   // timeout id while "confirm re-register" is armed

const errText = (e) => {
    if (e == null) return 'Lỗi không xác định';
    if (typeof e === 'string') return e;
    return e.message || String(e);
};

const hasWarp = (s) => !!(s && s.warp && s.warp.private_key);

/* ---------------- Settings <-> form ---------------- */

function fillForm(s) {
    el.modeInputs.forEach((i) => { i.checked = i.value === s.mode; });
    el.customLink.value = s.custom_link || '';
    el.doh.value = s.doh_server || '';
    el.dpi.checked = !!s.use_dpi;
    el.lan.checked = !!s.bypass_lan;
}

function readForm() {
    const checked = el.modeInputs.find((i) => i.checked);
    return {
        ...settings,
        mode: checked ? checked.value : (settings?.mode || 'bypass'),
        custom_link: el.customLink.value.trim(),
        doh_server: el.doh.value.trim(),
        use_dpi: el.dpi.checked,
        bypass_lan: el.lan.checked,
    };
}

function showSaveMsg(text, bad) {
    el.saveMsg.hidden = !text;
    el.saveMsg.textContent = text || '';
    el.saveMsg.classList.toggle('bad', !!bad);
}

function onFormChange() {
    if (!settings || status.running) return;
    settings = readForm();
    render();
    clearTimeout(saveTimer);
    saveTimer = setTimeout(() => flushSave().catch(() => {}), SAVE_DELAY);
}

async function flushSave() {
    clearTimeout(saveTimer);
    saveTimer = null;
    if (!settings || status.running) return;
    const snapshot = {...settings};
    const p = SaveSettings(snapshot)
        .then(() => showSaveMsg(''))
        .catch((e) => { showSaveMsg('Không lưu được: ' + errText(e), true); throw e; });
    savePromise = p;
    try { await p; } finally { if (savePromise === p) savePromise = null; }
}

/* ---------------- Rendering ---------------- */

function renderWarnings() {
    const parts = [];
    if (!status.admin) {
        parts.push('<p>Cần chạy với quyền Administrator.</p>');
    }
    if (!status.bins_ok) {
        const missing = (status.missing || []).map((m) => `<li><code>${escapeHtml(m)}</code></li>`).join('');
        parts.push(`<p>Thiếu file cần thiết${missing ? ':' : '.'}</p>${missing ? `<ul>${missing}</ul>` : ''}` +
            '<p>Hãy chạy <code>scripts\\fetch-bins.ps1</code> rồi mở lại ứng dụng.</p>');
    }
    el.warnings.hidden = parts.length === 0;
    el.warnings.innerHTML = parts.join('');
}

function render() {
    const running = !!status.running;
    const locked = running || busy;
    const mode = settings?.mode || 'bypass';
    const errMsg = callError || (!running ? status.error : '') || '';

    // Pill
    el.pill.classList.remove('on', 'off', 'err');
    if (running) { el.pill.classList.add('on'); el.pill.textContent = 'Đã kết nối'; }
    else if (errMsg) { el.pill.classList.add('err'); el.pill.textContent = 'Lỗi'; }
    else { el.pill.classList.add('off'); el.pill.textContent = 'Chưa kết nối'; }

    // Toggle
    el.toggle.classList.toggle('on', running);
    el.toggle.classList.toggle('busy', busy);
    el.toggle.disabled = busy || !settings;
    el.toggleLabel.textContent = busy
        ? (running ? 'Đang ngắt…' : 'Đang kết nối…')
        : (running ? 'Ngắt kết nối' : 'Kết nối');

    // Error
    el.error.hidden = !errMsg;
    el.error.textContent = errMsg;

    // Modes
    document.querySelectorAll('.mode').forEach((card) => {
        card.classList.toggle('sel', card.dataset.mode === mode);
        card.classList.toggle('dis', locked);
    });
    el.modeInputs.forEach((i) => { i.disabled = locked; });

    // WARP panel
    el.warpPanel.hidden = mode !== 'warp';
    const warp = hasWarp(settings);
    el.warpNone.hidden = warp;
    el.warpHas.hidden = !warp;
    el.warpRegister.disabled = locked || warpBusy;
    el.warpRegister.textContent = warpBusy ? 'Đang đăng ký…' : 'Đăng ký WARP';
    el.warpReregister.disabled = locked || warpBusy;
    if (!reregisterArmed) {
        el.warpReregister.textContent = warpBusy ? 'đang đăng ký…' : 'đăng ký lại';
    }

    // Custom panel
    el.customPanel.hidden = mode !== 'custom';
    el.customLink.disabled = locked;

    // Advanced
    el.doh.disabled = locked;
    el.dpi.disabled = locked;
    el.lan.disabled = locked;
    el.dpiRow.classList.toggle('dim', mode !== 'bypass');

    renderWarnings();
    renderDuration();
}

function renderDuration() {
    if (!status.running) { el.duration.innerHTML = '&nbsp;'; return; }
    const since = status.since ? Date.parse(status.since) : NaN;
    if (!since || Number.isNaN(since)) { el.duration.textContent = 'Đang chạy'; return; }
    let s = Math.max(0, Math.floor((Date.now() - since) / 1000));
    const h = Math.floor(s / 3600); s -= h * 3600;
    const m = Math.floor(s / 60); s -= m * 60;
    const pad = (n) => String(n).padStart(2, '0');
    el.duration.textContent = `Đã kết nối ${h > 0 ? h + ':' : ''}${pad(m)}:${pad(s)}`;
}

function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[c]));
}

/* ---------------- Status ---------------- */

function applyStatus(st) {
    if (!st) return;
    const wasRunning = status.running;
    status = {...status, ...st, missing: st.missing || []};
    // Once stopped, flush any pending edits that were blocked while running.
    if (wasRunning && !status.running && saveTimer) flushSave().catch(() => {});
    render();
}

async function pollStatus() {
    try { applyStatus(await GetStatus()); } catch (e) { console.error('GetStatus', e); }
}

/* ---------------- Actions ---------------- */

async function onToggle() {
    if (busy || !settings) return;
    busy = true;
    callError = '';
    render();
    try {
        if (status.running) {
            await Disconnect();
        } else {
            // make sure the backend has the latest settings before connecting
            if (saveTimer) await flushSave();
            else if (savePromise) await savePromise;
            await Connect();
        }
    } catch (e) {
        callError = errText(e);
    } finally {
        busy = false;
        await pollStatus();
        render();
    }
}

async function onRegisterWarp() {
    if (warpBusy || status.running || busy) return;
    disarmReregister();
    warpBusy = true;
    el.warpMsg.hidden = true;
    render();
    try {
        // persist pending edits first so the returned settings include them
        if (saveTimer) await flushSave().catch(() => {});
        const s = await RegisterWarp();
        if (s) {
            settings = s;
            fillForm(settings);
        }
        el.warpMsg.hidden = false;
        el.warpMsg.classList.remove('bad');
        el.warpMsg.textContent = 'Đăng ký WARP thành công.';
    } catch (e) {
        el.warpMsg.hidden = false;
        el.warpMsg.classList.add('bad');
        el.warpMsg.textContent = 'Đăng ký thất bại: ' + errText(e);
    } finally {
        warpBusy = false;
        render();
    }
}

function disarmReregister() {
    if (reregisterArmed) clearTimeout(reregisterArmed);
    reregisterArmed = null;
}

function onReregister() {
    if (reregisterArmed) { onRegisterWarp(); return; }
    el.warpReregister.textContent = 'nhấn lần nữa để xác nhận';
    reregisterArmed = setTimeout(() => { reregisterArmed = null; render(); }, 3000);
}

/* ---------------- Log ---------------- */

let logLines = 0;

function appendLog(line) {
    const box = el.log;
    const atBottom = box.scrollHeight - box.scrollTop - box.clientHeight < 24;
    box.appendChild(document.createTextNode(String(line).replace(/\r?\n$/, '') + '\n'));
    logLines++;
    while (logLines > LOG_MAX && box.firstChild) {
        box.removeChild(box.firstChild);
        logLines--;
    }
    el.logCount.textContent = `(${logLines})`;
    if (atBottom) box.scrollTop = box.scrollHeight;
}

el.logClear.addEventListener('click', (ev) => {
    ev.preventDefault();
    ev.stopPropagation();
    el.log.textContent = '';
    logLines = 0;
    el.logCount.textContent = '';
});
$('log-box').addEventListener('toggle', () => { el.log.scrollTop = el.log.scrollHeight; });

/* ---------------- Wiring ---------------- */

el.toggle.addEventListener('click', onToggle);
el.warpRegister.addEventListener('click', onRegisterWarp);
el.warpReregister.addEventListener('click', onReregister);
el.modeInputs.forEach((i) => i.addEventListener('change', onFormChange));
[el.dpi, el.lan].forEach((i) => i.addEventListener('change', onFormChange));
[el.doh, el.customLink].forEach((i) => i.addEventListener('input', onFormChange));

EventsOn('log', appendLog);
EventsOn('status', applyStatus);

async function init() {
    render();
    try {
        settings = await GetSettings();
        if (!settings.mode) settings.mode = 'bypass';
        fillForm(settings);
    } catch (e) {
        callError = 'Không đọc được cấu hình: ' + errText(e);
    }
    await pollStatus();
    render();
    setInterval(pollStatus, POLL_MS);
    setInterval(renderDuration, 1000);
}

init();

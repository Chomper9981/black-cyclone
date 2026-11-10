import './style.css';
import './app.css';

import {
    GetLogs,
    GetSettings,
    SaveSettings,
    GetStatus,
    Connect,
    Disconnect,
    Reconnect,
    RegisterWarp,
    ApplyWarpLicense,
    ScanWarpEndpoints,
    SetWarpEndpoint,
    AddServers,
    RemoveServer,
    RenameServer,
    AddSubscription,
    RefreshSubscriptions,
    RemoveSubscription,
    PingServers,
    CheckConnection,
    OpenDataFolder,
    Quit,
} from '../wailsjs/go/main/App';
import {EventsOn} from '../wailsjs/runtime/runtime';
import {initVPS, onShowVPS, renderVPS} from './vps.js';
import {initI18n, setLang, t, missingTranslations} from './i18n.js';

// Record the page's static text before anything renders into it. Until the backend
// reports the language in use, guess from the browser's.
initI18n();
setLang((navigator.language || '').toLowerCase().startsWith('vi') ? 'vi' : 'en');
window.__i18nMissing = missingTranslations;

const $ = (id) => document.getElementById(id);

const el = {
    pill: $('pill'),
    warnings: $('warnings'),
    cfgWarnings: $('cfg-warnings'),
    toggle: $('toggle'),
    toggleLabel: $('toggle-label'),
    duration: $('duration'),
    serverNow: $('server-now'),
    stats: $('stats'),
    stDown: $('st-down'),
    stUp: $('st-up'),
    stDownTotal: $('st-down-total'),
    stUpTotal: $('st-up-total'),
    error: $('error'),
    blocked: $('blocked'),
    blockedTitle: $('blocked-title'),
    blockedText: $('blocked-text'),
    blockedRelease: $('blocked-release'),
    blockedRetry: $('blocked-retry'),
    pending: $('pending'),
    pendingReconnect: $('pending-reconnect'),
    tabs: [...document.querySelectorAll('.tab')],
    panes: [...document.querySelectorAll('.pane')],
    saveMsg: $('save-msg'),

    modeInputs: [...document.querySelectorAll('input[name="mode"]')],
    // warp
    warpPanel: $('warp-panel'),
    warpNone: $('warp-none'),
    warpHas: $('warp-has'),
    warpType: $('warp-type'),
    warpRegister: $('warp-register'),
    warpReregister: $('warp-reregister'),
    warpKey: $('warp-key'),
    warpKeyApply: $('warp-key-apply'),
    warpEndpoint: $('warp-endpoint'),
    warpScan: $('warp-scan'),
    warpScanHint: $('warp-scan-hint'),
    warpScanList: $('warp-scan-list'),
    warpMsg: $('warp-msg'),
    // custom
    customPanel: $('custom-panel'),
    srvPing: $('srv-ping'),
    srvList: $('srv-list'),
    srvAdd: $('srv-add'),
    srvAddBtn: $('srv-add-btn'),
    srvMsg: $('srv-msg'),
    subPanel: $('sub-panel'),
    subRefresh: $('sub-refresh'),
    subList: $('sub-list'),
    subUrl: $('sub-url'),
    subName: $('sub-name'),
    subAddBtn: $('sub-add-btn'),
    subMsg: $('sub-msg'),
    // routing
    rDirectVN: $('r-direct-vn'),
    rDirectDomains: $('r-direct-domains'),
    rProxyDomains: $('r-proxy-domains'),
    rDirectProcs: $('r-direct-procs'),
    doh: $('doh'),
    lan: $('lan'),
    // app
    killSwitch: $('kill-switch'),
    autoReconnect: $('auto-reconnect'),
    autoConnect: $('auto-connect'),
    startWin: $('start-win'),
    startMin: $('start-min'),
    startMinRow: $('start-min-row'),
    closeTray: $('close-tray'),
    notify: $('notify'),
    langSelect: $('lang-select'),
    interruptSwitch: $('interrupt-switch'),
    openData: $('open-data'),
    quit: $('quit'),
    // tools
    checkBtn: $('check-btn'),
    checkOut: $('check-out'),
    log: $('log'),
    logCount: $('log-count'),
    logClear: $('log-clear'),
};

const LOG_MAX = 500;
const SAVE_DELAY = 500;
const POLL_MS = 3000;
const ARM_MS = 3000;

let settings = null;          // last known Settings (local edits applied)
let status = {
    running: false, state: 'off', mode: '', since: '', admin: true, bins_ok: true, missing: [],
    error: '', blocking: false, pending: false, server: '', warnings: [],
};
let stats = null;             // last "stats" event
let busy = false;             // Connect/Disconnect/Reconnect in flight
let callError = '';           // error from last rejected connect call
const ops = new Set();        // names of other in-flight operations

// debounced auto-save
let saveTimer = null;
let savePromise = null;
let dirty = false;            // local form edits not yet handed to SaveSettings
let actions = 0;              // settings-returning backend actions in flight

let pings = {};               // server id -> ms (-1 failed, -2 n/a)
let scanResults = null;       // [{ip, port, ms}]
let renaming = null;          // server id being renamed inline
const armed = new Map();      // key -> timeout id for 2-click confirmations

const errText = (e) => {
    if (e == null) return t('Lỗi không xác định', 'Unknown error');
    if (typeof e === 'string') return e;
    return e.message || String(e);
};

function escapeHtml(s) {
    return String(s ?? '').replace(/[&<>"']/g, (c) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[c]));
}

const hasWarp = (s) => !!(s && s.warp && s.warp.private_key);
const isBusy = (op) => ops.has(op);

function normalize(s) {
    s = {...(s || {})};
    if (s.mode !== 'warp' && s.mode !== 'custom') s.mode = 'warp';
    if (!s.selected_server) s.selected_server = 'auto';
    if (s.language !== 'vi' && s.language !== 'en') s.language = ''; // '' = follow the system
    s.servers = s.servers || [];
    s.subscriptions = s.subscriptions || [];
    const r = s.routing || {};
    s.routing = {
        direct_vn: !!r.direct_vn,
        direct_domains: r.direct_domains || [],
        proxy_domains: r.proxy_domains || [],
        direct_processes: r.direct_processes || [],
    };
    return s;
}

function fmtBytes(n) {
    n = Math.max(0, Number(n) || 0);
    if (n < 1024) return `${n} B`;
    const units = ['KB', 'MB', 'GB', 'TB'];
    let i = -1;
    do { n /= 1024; i++; } while (n >= 1024 && i < units.length - 1);
    return `${n < 10 ? n.toFixed(1) : Math.round(n)} ${units[i]}`;
}

function fmtAgo(iso) {
    const ts = Date.parse(iso || '');
    if (!ts) return t('chưa cập nhật', 'never updated');
    const s = Math.max(0, Math.floor((Date.now() - ts) / 1000));
    if (s < 60) return t('vừa xong', 'just now');
    if (s < 3600) return t(`${Math.floor(s / 60)} phút trước`, `${Math.floor(s / 60)} min ago`);
    if (s < 86400) return t(`${Math.floor(s / 3600)} giờ trước`, `${Math.floor(s / 3600)} h ago`);
    return t(`${Math.floor(s / 86400)} ngày trước`, `${Math.floor(s / 86400)} d ago`);
}

function protoOf(link) {
    const m = /^([a-z0-9]+):\/\//i.exec(String(link || '').trim());
    const p = m ? m[1].toLowerCase() : '';
    return ({vless: 'VLESS', vmess: 'VMess', trojan: 'Trojan', ss: 'SS', hysteria2: 'HY2', hy2: 'HY2', tuic: 'TUIC'})[p] || (p ? p.toUpperCase() : '?');
}

const hostPort = (ip, port) => (String(ip).includes(':') ? `[${ip}]` : ip) + (port ? `:${port}` : '');

const lines = (text) => text.split(/\r?\n/).map((l) => l.trim()).filter(Boolean);

// setHTML only touches the DOM when the markup changed (keeps focus/hover stable
// across the periodic status re-render). Returns whether it changed.
function setHTML(node, html) {
    if (node._html === html) return false;
    node._html = html;
    node.innerHTML = html;
    return true;
}

function setMsg(node, text, bad) {
    node.hidden = !text;
    node.textContent = text || '';
    node.classList.toggle('bad', !!bad);
}

/* ---------------- 2-click confirmation ---------------- */

function arm(key) {
    disarm(key);
    armed.set(key, setTimeout(() => { armed.delete(key); render(); }, ARM_MS));
    render();
}

function disarm(key) {
    if (armed.has(key)) clearTimeout(armed.get(key));
    armed.delete(key);
}

/* ---------------- Settings <-> form ---------------- */

function fillForm(s) {
    el.modeInputs.forEach((i) => { i.checked = i.value === s.mode; });
    el.langSelect.value = s.language || '';
    el.doh.value = s.doh_server || '';
    el.lan.checked = !!s.bypass_lan;
    el.rDirectVN.checked = !!s.routing.direct_vn;
    el.rDirectDomains.value = s.routing.direct_domains.join('\n');
    el.rProxyDomains.value = s.routing.proxy_domains.join('\n');
    el.rDirectProcs.value = s.routing.direct_processes.join('\n');
    el.killSwitch.checked = !!s.kill_switch;
    el.autoReconnect.checked = !!s.auto_reconnect;
    el.autoConnect.checked = !!s.auto_connect;
    el.startWin.checked = !!s.start_with_windows;
    el.startMin.checked = !!s.start_minimized;
    el.closeTray.checked = !!s.close_to_tray;
    el.notify.checked = !!s.notifications;
    el.interruptSwitch.checked = !!s.interrupt_on_switch;
}

// readForm overlays the form-owned fields onto settings. Servers, subscriptions,
// selected_server and warp are owned by backend actions / direct clicks.
function readForm() {
    const checked = el.modeInputs.find((i) => i.checked);
    return {
        ...settings,
        mode: checked ? checked.value : (settings?.mode || 'warp'),
        language: el.langSelect.value,
        doh_server: el.doh.value.trim(),
        bypass_lan: el.lan.checked,
        routing: {
            direct_vn: el.rDirectVN.checked,
            direct_domains: lines(el.rDirectDomains.value),
            proxy_domains: lines(el.rProxyDomains.value),
            direct_processes: lines(el.rDirectProcs.value),
        },
        kill_switch: el.killSwitch.checked,
        auto_reconnect: el.autoReconnect.checked,
        auto_connect: el.autoConnect.checked,
        start_with_windows: el.startWin.checked,
        start_minimized: el.startMin.checked,
        close_to_tray: el.closeTray.checked,
        notifications: el.notify.checked,
        interrupt_on_switch: el.interruptSwitch.checked,
    };
}

function scheduleSave() {
    dirty = true;
    clearTimeout(saveTimer);
    saveTimer = setTimeout(() => flushSave().catch(() => {}), SAVE_DELAY);
}

function onFormChange() {
    if (!settings) return;
    settings = readForm();
    scheduleSave();
    render();
}

// flushSave hands pending edits to the backend and waits for every save in flight.
// While a settings action runs, saving is deferred (the action's result is merged
// with the edits afterwards) so stale data never overwrites backend changes.
async function flushSave() {
    clearTimeout(saveTimer);
    saveTimer = null;
    if (actions > 0) return;
    if (savePromise) { try { await savePromise; } catch { /* reported already */ } }
    if (!settings || !dirty) return;
    dirty = false;
    const snapshot = JSON.parse(JSON.stringify(settings));
    const p = SaveSettings(snapshot)
        .then(() => setMsg(el.saveMsg, ''))
        .catch((e) => { setMsg(el.saveMsg, t('Không lưu được: ', 'Could not save: ') + errText(e), true); throw e; });
    savePromise = p;
    try { await p; } finally { if (savePromise === p) savePromise = null; }
}

// adopt replaces local settings with the backend's copy, keeping unsaved form edits.
function adopt(s) {
    if (!s) return;
    settings = normalize(s);
    if (dirty) settings = readForm();
    fillForm(settings);
}

// runAction runs a backend call that returns the full updated Settings.
async function runAction(op, fn) {
    ops.add(op);
    render();
    try {
        await flushSave().catch(() => {});
        actions++;
        try {
            const s = await fn();
            adopt(s);
            return s;
        } catch (e) {
            // Some actions change settings and still report an error (e.g. a
            // subscription refresh where one feed failed): re-read to stay in sync.
            try { adopt(await GetSettings()); } catch { /* keep local copy */ }
            throw e;
        } finally {
            actions--;
        }
    } finally {
        ops.delete(op);
        if (dirty && actions === 0) scheduleSave();
        render();
    }
}

/* ---------------- Rendering ---------------- */

function renderWarnings() {
    const parts = [];
    const linux = status.os === 'linux';
    if (!status.admin) {
        parts.push(linux
            ? t('<p>App chưa có quyền tạo card mạng ảo. Cài bằng <code>linux/install.sh</code>, hoặc chạy ' +
                '<code>sudo setcap cap_net_admin,cap_net_raw+ep</code> cho file BlackCyclone rồi mở lại.</p>',
                '<p>The app may not create the virtual network device. Install it with <code>linux/install.sh</code>, or run ' +
                '<code>sudo setcap cap_net_admin,cap_net_raw+ep</code> on the BlackCyclone binary and open it again.</p>')
            : t('<p>Cần chạy với quyền Administrator.</p>', '<p>The app must run as Administrator.</p>'));
    }
    if (!status.bins_ok) {
        const missing = (status.missing || []).map((m) => `<li><code>${escapeHtml(m)}</code></li>`).join('');
        const script = linux ? 'linux/fetch-bins.sh' : 'windows\\fetch-bins.ps1';
        parts.push(`<p>${t('Thiếu file cần thiết', 'Required files are missing')}${missing ? ':' : '.'}</p>${missing ? `<ul>${missing}</ul>` : ''}` +
            t(`<p>Hãy chạy <code>${script}</code> rồi mở lại ứng dụng.</p>`, `<p>Run <code>${script}</code>, then open the app again.</p>`));
    }
    el.warnings.hidden = parts.length === 0;
    el.warnings.innerHTML = parts.join('');

    const w = status.running ? (status.warnings || []) : [];
    el.cfgWarnings.hidden = w.length === 0;
    el.cfgWarnings.innerHTML = w.map((x) => `<p>${escapeHtml(x)}</p>`).join('');
}

function connState() {
    if (!status.running) return 'off';
    return status.state || 'on';
}

function renderHero() {
    const state = connState();
    const errMsg = callError || (state !== 'on' ? status.error : '') || '';

    el.pill.classList.remove('on', 'off', 'err', 'warn');
    const pill = {
        on: ['on', t('Đã kết nối', 'Connected')],
        reconnecting: ['warn', t('Đang kết nối lại…', 'Reconnecting…')],
        blocked: ['err', t('Đang chặn mạng', 'Network blocked')],
    }[state] || (errMsg ? ['err', t('Lỗi', 'Error')] : ['off', t('Chưa kết nối', 'Not connected')]);
    el.pill.classList.add(pill[0]);
    el.pill.textContent = pill[1];

    el.toggle.classList.toggle('on', state === 'on');
    el.toggle.classList.toggle('warn', state === 'reconnecting');
    el.toggle.classList.toggle('blocked', state === 'blocked');
    el.toggle.classList.toggle('busy', busy || state === 'reconnecting');
    el.toggle.disabled = busy || !settings;
    el.toggleLabel.textContent = busy
        ? (status.running ? t('Đang ngắt…', 'Disconnecting…') : t('Đang kết nối…', 'Connecting…'))
        : (status.running ? t('Ngắt kết nối', 'Disconnect') : t('Kết nối', 'Connect'));

    el.error.hidden = !errMsg;
    el.error.textContent = errMsg;

    // Kill switch banner
    const blocking = status.running && (state === 'blocked' || status.blocking);
    el.blocked.hidden = !blocking;
    if (blocking) {
        el.blocked.classList.toggle('amber', state === 'reconnecting');
        el.blockedTitle.textContent = state === 'reconnecting'
            ? t('Đang kết nối lại — kill switch tạm chặn mạng', 'Reconnecting — the kill switch is blocking the network meanwhile')
            : t('Kill switch đang chặn mạng', 'The kill switch is blocking the network');
        el.blockedText.textContent = state === 'reconnecting'
            ? t('VPN bị rớt. Internet bị chặn để không lộ IP thật cho tới khi kết nối lại được.', 'The VPN dropped. The Internet is blocked to keep your real IP hidden until it reconnects.')
            : t('VPN bị rớt và không kết nối lại được. Mọi kết nối Internet đang bị chặn để không lộ IP thật. Bấm “Kết nối lại” hoặc ngắt để dùng mạng bình thường.', 'The VPN dropped and could not reconnect. All Internet connections are blocked to keep your real IP hidden. Click “Reconnect”, or disconnect to use the regular network.');
        el.blockedRelease.disabled = busy;
        el.blockedRetry.disabled = busy;
        el.blockedRetry.hidden = state === 'reconnecting';
    }

    // Pending
    el.pending.hidden = !(state === 'on' && status.pending);
    el.pendingReconnect.disabled = busy;
    el.pendingReconnect.textContent = busy ? t('Đang kết nối lại…', 'Reconnecting…') : t('Kết nối lại', 'Reconnect');

    // Server in use
    const showServer = state === 'on' && status.mode === 'custom' && status.server;
    el.serverNow.hidden = !showServer;
    if (showServer) el.serverNow.textContent = status.server;

    renderStats();
    renderDuration();
}

function renderStats() {
    const show = connState() === 'on' && !!stats;
    el.stats.hidden = !show;
    if (!show) return;
    el.stDown.textContent = fmtBytes(stats.down) + '/s';
    el.stUp.textContent = fmtBytes(stats.up) + '/s';
    el.stDownTotal.textContent = fmtBytes(stats.down_total);
    el.stUpTotal.textContent = fmtBytes(stats.up_total);
}

function renderDuration() {
    const state = connState();
    if (state === 'reconnecting') { el.duration.textContent = t('Mất kết nối, đang thử lại…', 'Connection lost, retrying…'); return; }
    if (state === 'blocked') { el.duration.textContent = t('Mất kết nối', 'Connection lost'); return; }
    if (state !== 'on') { el.duration.innerHTML = '&nbsp;'; return; }
    const since = status.since ? Date.parse(status.since) : NaN;
    if (!since || Number.isNaN(since)) { el.duration.textContent = t('Đang chạy', 'Running'); return; }
    let s = Math.max(0, Math.floor((Date.now() - since) / 1000));
    const h = Math.floor(s / 3600); s -= h * 3600;
    const m = Math.floor(s / 60); s -= m * 60;
    const pad = (n) => String(n).padStart(2, '0');
    el.duration.textContent = `${t('Đã kết nối', 'Connected')} ${h > 0 ? h + ':' : ''}${pad(m)}:${pad(s)}`;
}

function renderModes() {
    const mode = settings?.mode || 'warp';
    document.querySelectorAll('.mode').forEach((card) => {
        card.classList.toggle('sel', card.dataset.mode === mode);
    });
    el.warpPanel.hidden = mode !== 'warp';
    el.customPanel.hidden = mode !== 'custom';
    el.subPanel.hidden = mode !== 'custom';
}

function warpTypeLabel(type) { // not "t": that is the translation function
    switch (type) {
    case 'unlimited': return ['WARP+', 'plus'];
    case 'limited': return [t('WARP+ (giới hạn)', 'WARP+ (limited)'), 'plus'];
    case 'free': case '': case undefined: return [t('Miễn phí', 'Free'), ''];
    default: return [type, ''];
    }
}

function renderWarp() {
    if (!settings || settings.mode !== 'warp') return;
    const w = settings.warp;
    const has = hasWarp(settings);
    el.warpNone.hidden = has;
    el.warpHas.hidden = !has;

    const regBusy = isBusy('register');
    el.warpRegister.disabled = regBusy;
    el.warpRegister.textContent = regBusy ? t('Đang đăng ký…', 'Registering…') : t('Đăng ký WARP', 'Register WARP');
    el.warpReregister.disabled = regBusy;
    el.warpReregister.textContent = regBusy ? t('đang đăng ký…', 'registering…')
        : armed.has('rereg') ? t('nhấn lần nữa để xác nhận', 'click again to confirm') : t('đăng ký lại', 'register again');
    if (!has) return;

    const [label, cls] = warpTypeLabel(w.account_type);
    el.warpType.textContent = label;
    el.warpType.className = 'badge ' + cls;

    const licBusy = isBusy('license');
    el.warpKeyApply.disabled = licBusy || !el.warpKey.value.trim();
    el.warpKeyApply.textContent = licBusy ? t('Đang áp dụng…', 'Applying…') : t('Áp dụng', 'Apply');

    el.warpEndpoint.textContent = w.endpoint ? hostPort(w.endpoint, w.endpoint_port) : t('Mặc định', 'Default');

    const tunneled = status.running;
    const scanBusy = isBusy('scan');
    el.warpScan.disabled = tunneled || scanBusy;
    el.warpScan.textContent = scanBusy ? t('Đang quét… (~30s)', 'Scanning… (~30s)') : t('Quét endpoint', 'Scan endpoints');
    el.warpScanHint.textContent = tunneled ? t('Ngắt kết nối trước khi quét', 'Disconnect before scanning') : '';

    el.warpScanList.hidden = !scanResults;
    if (scanResults) {
        if (scanResults.length === 0) {
            setHTML(el.warpScanList, `<div class="empty">${t('Không có endpoint nào phản hồi.', 'No endpoint answered.')}</div>`);
        } else {
            const epBusy = isBusy('endpoint');
            setHTML(el.warpScanList, scanResults.map((r, i) => {
                const cur = w.endpoint === r.ip && Number(w.endpoint_port) === Number(r.port);
                return `<button type="button" class="pick${cur ? ' sel' : ''}" data-i="${i}"${epBusy ? ' disabled' : ''}>` +
                    `<code>${escapeHtml(hostPort(r.ip, r.port))}</code>` +
                    `<span class="ms ${msClass(r.ms)}">${escapeHtml(r.ms)} ms</span></button>`;
            }).join(''));
        }
    }
}

function msClass(ms) {
    if (ms < 0) return 'bad';
    if (ms < 150) return 'good';
    if (ms < 400) return 'mid';
    return 'slow';
}

function pingBadge(id) {
    if (!(id in pings)) return '';
    const ms = pings[id];
    if (ms === -1) return `<span class="ms bad">${t('lỗi', 'failed')}</span>`;
    if (ms === -2) return `<span class="ms na" title="${t('Không đo được giao thức UDP khi chưa kết nối', 'UDP protocols cannot be measured while disconnected')}">—</span>`;
    return `<span class="ms ${msClass(ms)}">${escapeHtml(ms)} ms</span>`;
}

function renderServers() {
    if (!settings || settings.mode !== 'custom') return;
    // Don't rebuild while the user is typing a new name.
    if (renaming && el.srvList.contains(document.activeElement) && document.activeElement.tagName === 'INPUT') {
        return;
    }
    const sel = settings.selected_server || 'auto';
    const subs = new Map(settings.subscriptions.map((s) => [s.id, s.name]));
    const rows = [];
    rows.push(
        `<label class="srv${sel === 'auto' ? ' sel' : ''}">` +
        `<input type="radio" name="srv" value="auto"${sel === 'auto' ? ' checked' : ''}>` +
        `<span class="srv-main"><span class="srv-name">${t('Tự động (nhanh nhất)', 'Auto (fastest)')}</span>` +
        `<span class="srv-meta">${t('Chọn server có độ trễ thấp nhất', 'Uses the server with the lowest latency')}</span></span></label>`);
    for (const s of settings.servers) {
        const id = escapeHtml(s.id);
        const isSel = sel === s.id;
        const delArmed = armed.has('del:' + s.id);
        const from = s.sub_id ? (subs.get(s.sub_id) || 'subscription') : '';
        let main;
        if (renaming === s.id) {
            main = `<span class="srv-main"><input class="srv-rename" type="text" data-id="${id}" value="${escapeHtml(s.name)}" spellcheck="false"></span>`;
        } else {
            main = '<span class="srv-main">' +
                `<span class="srv-name" title="${escapeHtml(s.name)}">${escapeHtml(s.name || t('(không tên)', '(no name)'))}</span>` +
                `<span class="srv-meta"><span class="proto">${escapeHtml(protoOf(s.link))}</span>` +
                (from ? `<span class="from">${escapeHtml(from)}</span>` : '') + '</span></span>';
        }
        rows.push(
            `<label class="srv${isSel ? ' sel' : ''}">` +
            `<input type="radio" name="srv" value="${id}"${isSel ? ' checked' : ''}>` +
            main +
            pingBadge(s.id) +
            '<span class="srv-acts">' +
            `<button type="button" class="icon" data-act="rename" data-id="${id}" title="${t('Đổi tên', 'Rename')}" aria-label="${t('Đổi tên', 'Rename')}">✎</button>` +
            `<button type="button" class="icon del${delArmed ? ' armed' : ''}" data-act="del" data-id="${id}" ` +
            `title="${delArmed ? t('Nhấn lần nữa để xoá', 'Click again to remove') : t('Xoá', 'Remove')}" aria-label="${t('Xoá', 'Remove')}">${delArmed ? t('xoá?', 'remove?') : '✕'}</button>` +
            '</span></label>');
    }
    if (settings.servers.length === 0) {
        rows.push(`<div class="empty">${t('Chưa có server. Thêm link hoặc subscription bên dưới.', 'No servers yet. Add a link or a subscription below.')}</div>`);
    }
    const changed = setHTML(el.srvList, rows.join(''));
    if (renaming && changed) {
        const inp = el.srvList.querySelector('.srv-rename');
        if (inp) { inp.focus(); inp.select(); }
    }

    const pingBusy = isBusy('ping');
    el.srvPing.disabled = pingBusy || settings.servers.length === 0;
    el.srvPing.textContent = pingBusy ? t('đang ping…', 'pinging…') : 'ping';
    const addBusy = isBusy('add');
    el.srvAddBtn.disabled = addBusy;
    el.srvAddBtn.textContent = addBusy ? t('Đang thêm…', 'Adding…') : t('Thêm', 'Add');
}

function renderSubs() {
    if (!settings || settings.mode !== 'custom') return;
    const counts = new Map();
    for (const s of settings.servers) if (s.sub_id) counts.set(s.sub_id, (counts.get(s.sub_id) || 0) + 1);
    const subs = settings.subscriptions;
    setHTML(el.subList, subs.length === 0
        ? `<div class="empty">${t('Chưa có subscription.', 'No subscriptions yet.')}</div>`
        : subs.map((s) => {
            const delArmed = armed.has('sub:' + s.id);
            return '<div class="sub-row">' +
                `<span class="srv-main"><span class="srv-name" title="${escapeHtml(s.url)}">${escapeHtml(s.name)}</span>` +
                `<span class="srv-meta">${counts.get(s.id) || 0} server · ${escapeHtml(fmtAgo(s.updated))}</span></span>` +
                `<button type="button" class="icon del${delArmed ? ' armed' : ''}" data-act="sub-del" data-id="${escapeHtml(s.id)}" ` +
                `title="${delArmed ? t('Nhấn lần nữa để xoá', 'Click again to remove') : t('Xoá subscription và server của nó', 'Remove the subscription and its servers')}" aria-label="${t('Xoá', 'Remove')}">${delArmed ? t('xoá?', 'remove?') : '✕'}</button>` +
                '</div>';
        }).join(''));
    const refBusy = isBusy('sub-refresh');
    el.subRefresh.disabled = refBusy || subs.length === 0;
    el.subRefresh.textContent = refBusy ? t('đang cập nhật…', 'updating…') : t('cập nhật tất cả', 'update all');
    const addBusy = isBusy('sub-add');
    el.subAddBtn.disabled = addBusy;
    el.subAddBtn.textContent = addBusy ? t('Đang tải…', 'Downloading…') : t('Thêm', 'Add');
}

function renderOptions() {
    el.startMinRow.classList.toggle('dim', !el.startWin.checked);
    el.startMin.disabled = !el.startWin.checked;

    const checkBusy = isBusy('check');
    el.checkBtn.disabled = checkBusy;
    el.checkBtn.textContent = checkBusy ? t('Đang kiểm tra…', 'Checking…') : t('Kiểm tra kết nối', 'Check connection');
}

function render() {
    renderHero();
    renderModes();
    renderWarp();
    renderServers();
    renderSubs();
    renderOptions();
    renderWarnings();
    renderVPS();
}

/* ---------------- Status & stats ---------------- */

function applyStatus(st) {
    if (!st) return;
    status = {...status, ...st, missing: st.missing || [], warnings: st.warnings || []};
    applyOS(status.os);
    setLang(status.lang); // the backend resolves "auto" to the system language
    if (connState() !== 'on') stats = null;
    render();
}

// applyOS switches the OS-specific texts (.os-win / .os-linux, data-linux*) once.
function applyOS(os) {
    if (!os || document.body.dataset.os === os) return;
    document.body.dataset.os = os;
    if (os !== 'linux') return;
    for (const t of document.querySelectorAll('[data-linux-placeholder]')) t.placeholder = t.dataset.linuxPlaceholder;
}

async function pollStatus() {
    try { applyStatus(await GetStatus()); } catch (e) { console.error('GetStatus', e); }
}

function applyStats(s) {
    if (!s) return;
    stats = s;
    renderStats();
}

/* ---------------- Connection actions ---------------- */

async function connCall(fn) {
    if (busy || !settings) return;
    busy = true;
    callError = '';
    render();
    try {
        await fn();
    } catch (e) {
        callError = errText(e);
    } finally {
        busy = false;
        await pollStatus();
        render();
    }
}

async function persistBeforeConnect() {
    await flushSave();
}

function onToggle() {
    connCall(async () => {
        if (status.running) {
            await Disconnect();
        } else {
            await persistBeforeConnect();
            await Connect();
        }
    });
}

function onReconnect() {
    connCall(async () => {
        await persistBeforeConnect();
        await Reconnect();
    });
}

/* ---------------- WARP ---------------- */

async function onRegisterWarp() {
    if (isBusy('register')) return;
    disarm('rereg');
    setMsg(el.warpMsg, '');
    const before = settings?.warp?.private_key || '';
    try {
        const s = await runAction('register', RegisterWarp);
        // RegisterWarp reports failure only in the log and returns the old settings.
        if (s && s.warp && s.warp.private_key && s.warp.private_key !== before) {
            setMsg(el.warpMsg, t('Đăng ký WARP thành công.', 'WARP registered.') + (status.running && status.mode === 'warp' ? t(' Kết nối lại để dùng tài khoản mới.', ' Reconnect to use the new account.') : ''));
            scanResults = null;
        } else {
            setMsg(el.warpMsg, t('Đăng ký thất bại — xem nhật ký để biết chi tiết.', 'Registration failed — see the log for details.'), true);
        }
    } catch (e) {
        setMsg(el.warpMsg, t('Đăng ký thất bại: ', 'Registration failed: ') + errText(e), true);
    }
    render();
}

function onReregister() {
    if (armed.has('rereg')) { onRegisterWarp(); return; }
    arm('rereg');
}

async function onApplyLicense() {
    const key = el.warpKey.value.trim();
    if (!key || isBusy('license')) return;
    setMsg(el.warpMsg, '');
    try {
        const s = await runAction('license', () => ApplyWarpLicense(key));
        el.warpKey.value = '';
        setMsg(el.warpMsg, t('Đã áp dụng key. Loại tài khoản: ', 'Key applied. Account type: ') + warpTypeLabel(s?.warp?.account_type)[0] + '.');
    } catch (e) {
        setMsg(el.warpMsg, t('Không áp dụng được key: ', 'Could not apply the key: ') + errText(e), true);
    }
    render();
}

async function onScan() {
    if (isBusy('scan')) return;
    setMsg(el.warpMsg, '');
    ops.add('scan');
    render();
    try {
        scanResults = (await ScanWarpEndpoints()) || [];
    } catch (e) {
        setMsg(el.warpMsg, t('Quét thất bại: ', 'Scan failed: ') + errText(e), true);
    } finally {
        ops.delete('scan');
        render();
    }
}

async function onPickEndpoint(ev) {
    const b = ev.target.closest('.pick');
    if (!b || !scanResults || isBusy('endpoint')) return;
    const r = scanResults[Number(b.dataset.i)];
    if (!r) return;
    try {
        await runAction('endpoint', () => SetWarpEndpoint(r.ip, r.port));
        setMsg(el.warpMsg, t(`Đã chọn endpoint ${hostPort(r.ip, r.port)}.`, `Endpoint ${hostPort(r.ip, r.port)} selected.`));
    } catch (e) {
        setMsg(el.warpMsg, t('Không đặt được endpoint: ', 'Could not set the endpoint: ') + errText(e), true);
    }
    render();
}

/* ---------------- Servers ---------------- */

function onServerSelect(ev) {
    const t = ev.target;
    if (t.name !== 'srv' || !settings) return;
    settings = {...settings, selected_server: t.value};
    scheduleSave();
    render();
}

function onServerClick(ev) {
    const b = ev.target.closest('button[data-act]');
    if (!b) return;
    ev.preventDefault();
    ev.stopPropagation();
    const id = b.dataset.id;
    if (b.dataset.act === 'rename') {
        renaming = id;
        render();
    } else if (b.dataset.act === 'del') {
        const key = 'del:' + id;
        if (!armed.has(key)) { arm(key); return; }
        disarm(key);
        delete pings[id];
        runAction('del', () => RemoveServer(id))
            .then(() => setMsg(el.srvMsg, ''))
            .catch((e) => setMsg(el.srvMsg, t('Không xoá được: ', 'Could not remove: ') + errText(e), true));
    }
}

let renameCommitting = false;

async function commitRename(inp, save) {
    if (renameCommitting || renaming == null) return;
    renameCommitting = true;
    const id = inp.dataset.id;
    const name = inp.value.trim();
    const old = settings.servers.find((s) => s.id === id);
    renaming = null;
    try {
        if (save && name && old && name !== old.name) {
            await runAction('rename', () => RenameServer(id, name));
        }
    } catch (e) {
        setMsg(el.srvMsg, t('Không đổi tên được: ', 'Could not rename: ') + errText(e), true);
    } finally {
        renameCommitting = false;
        render();
    }
}

function onRenameKey(ev) {
    if (!ev.target.classList.contains('srv-rename')) return;
    if (ev.key === 'Enter') { ev.preventDefault(); commitRename(ev.target, true); }
    else if (ev.key === 'Escape') { ev.preventDefault(); commitRename(ev.target, false); }
}

function onRenameBlur(ev) {
    if (ev.target.classList && ev.target.classList.contains('srv-rename')) commitRename(ev.target, true);
}

async function onAddServers() {
    const text = el.srvAdd.value.trim();
    if (!text || isBusy('add')) return;
    setMsg(el.srvMsg, '');
    const before = settings.servers.length;
    try {
        const s = await runAction('add', () => AddServers(text));
        const added = (s?.servers?.length || 0) - before;
        if (added > 0) {
            el.srvAdd.value = '';
            setMsg(el.srvMsg, t(`Đã thêm ${added} server.`, `Added ${added} server(s).`));
        } else {
            setMsg(el.srvMsg, t('Không thêm được server nào (link không hỗ trợ hoặc đã có).', 'No server was added (unsupported link, or already in the list).'), true);
        }
    } catch (e) {
        setMsg(el.srvMsg, errText(e), true);
    }
    render();
}

async function onPing() {
    if (isBusy('ping')) return;
    ops.add('ping');
    setMsg(el.srvMsg, '');
    render();
    try {
        pings = (await PingServers()) || {};
    } catch (e) {
        setMsg(el.srvMsg, t('Ping thất bại: ', 'Ping failed: ') + errText(e), true);
    } finally {
        ops.delete('ping');
        render();
    }
}

/* ---------------- Subscriptions ---------------- */

async function onAddSub() {
    const url = el.subUrl.value.trim();
    if (!url || isBusy('sub-add')) return;
    if (!/^https?:\/\//i.test(url)) {
        setMsg(el.subMsg, t('URL phải bắt đầu bằng http:// hoặc https://', 'The URL must start with http:// or https://'), true);
        return;
    }
    setMsg(el.subMsg, '');
    try {
        await runAction('sub-add', () => AddSubscription(url, el.subName.value.trim()));
        el.subUrl.value = '';
        el.subName.value = '';
        setMsg(el.subMsg, t('Đã thêm subscription.', 'Subscription added.'));
    } catch (e) {
        setMsg(el.subMsg, t('Không tải được subscription: ', 'Could not download the subscription: ') + errText(e), true);
    }
    render();
}

async function onRefreshSubs() {
    if (isBusy('sub-refresh')) return;
    setMsg(el.subMsg, '');
    try {
        await runAction('sub-refresh', RefreshSubscriptions);
        setMsg(el.subMsg, t('Đã cập nhật.', 'Updated.'));
    } catch (e) {
        setMsg(el.subMsg, t('Cập nhật lỗi: ', 'Update failed: ') + errText(e), true);
    }
    render();
}

function onSubClick(ev) {
    const b = ev.target.closest('button[data-act="sub-del"]');
    if (!b) return;
    const id = b.dataset.id;
    const key = 'sub:' + id;
    if (!armed.has(key)) { arm(key); return; }
    disarm(key);
    runAction('sub-del', () => RemoveSubscription(id))
        .then(() => setMsg(el.subMsg, ''))
        .catch((e) => setMsg(el.subMsg, t('Không xoá được: ', 'Could not remove: ') + errText(e), true));
}

/* ---------------- Tools ---------------- */

const warpFlag = (v) => ({on: 'WARP', plus: 'WARP+', off: t('không', 'off')}[v]);

async function onCheck() {
    if (isBusy('check')) return;
    ops.add('check');
    render();
    try {
        const r = await CheckConnection();
        const ip = r?.ip || {};
        const head = ip.error
            ? `<div class="ip-err">${t('Không lấy được IP', 'Could not get the IP')}: ${escapeHtml(ip.error)}</div>`
            : '<div class="ip-grid">' +
              `<span>IP</span><code>${escapeHtml(ip.ip || '?')}</code>` +
              `<span>${t('Quốc gia', 'Country')}</span><b>${escapeHtml(ip.country || '?')}</b>` +
              `<span>Cloudflare</span><b>${escapeHtml(ip.colo || '?')}</b>` +
              `<span>WARP</span><b>${escapeHtml(warpFlag(ip.warp) || ip.warp || '?')}</b></div>`;
        const sites = (r?.sites || []).map((s) =>
            `<div class="site ${s.ok ? 'ok' : 'bad'}">` +
            `<span class="mark">${s.ok ? '✓' : '✗'}</span>` +
            `<span class="site-name">${escapeHtml(s.name)}</span>` +
            (s.ok ? `<span class="ms ${msClass(s.ms)}">${escapeHtml(s.ms)} ms</span>`
                  : `<span class="site-err" title="${escapeHtml(s.error)}">${escapeHtml(s.error || ('HTTP ' + s.status))}</span>`) +
            '</div>').join('');
        el.checkOut.innerHTML = head + `<div class="sites">${sites}</div>`;
    } catch (e) {
        el.checkOut.innerHTML = `<div class="ip-err">${escapeHtml(errText(e))}</div>`;
    } finally {
        el.checkOut.hidden = false;
        ops.delete('check');
        render();
    }
}

/* ---------------- Tabs ---------------- */

function showTab(name) {
    if (!el.panes.some((p) => p.dataset.pane === name)) name = 'conn';
    el.tabs.forEach((t) => {
        const on = t.dataset.tab === name;
        t.classList.toggle('sel', on);
        t.setAttribute('aria-selected', on ? 'true' : 'false');
    });
    el.panes.forEach((p) => { p.hidden = p.dataset.pane !== name; });
    if (name === 'tools') el.log.scrollTop = el.log.scrollHeight;
    if (name === 'vps') onShowVPS();
    try { localStorage.setItem('rcv-tab', name); } catch { /* ignore */ }
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
el.blockedRelease.addEventListener('click', () => connCall(() => Disconnect()));
el.pendingReconnect.addEventListener('click', onReconnect);
el.blockedRetry.addEventListener('click', onReconnect);
el.tabs.forEach((t) => t.addEventListener('click', () => showTab(t.dataset.tab)));

el.warpRegister.addEventListener('click', onRegisterWarp);
el.warpReregister.addEventListener('click', onReregister);
el.warpKey.addEventListener('input', renderWarp);
el.warpKey.addEventListener('keydown', (ev) => { if (ev.key === 'Enter') onApplyLicense(); });
el.warpKeyApply.addEventListener('click', onApplyLicense);
el.warpScan.addEventListener('click', onScan);
el.warpScanList.addEventListener('click', onPickEndpoint);

el.srvList.addEventListener('change', onServerSelect);
el.srvList.addEventListener('click', onServerClick);
el.srvList.addEventListener('keydown', onRenameKey);
el.srvList.addEventListener('focusout', onRenameBlur);
el.srvPing.addEventListener('click', onPing);
el.srvAddBtn.addEventListener('click', onAddServers);

el.subList.addEventListener('click', onSubClick);
el.subRefresh.addEventListener('click', onRefreshSubs);
el.subAddBtn.addEventListener('click', onAddSub);
el.subUrl.addEventListener('keydown', (ev) => { if (ev.key === 'Enter') onAddSub(); });

el.checkBtn.addEventListener('click', onCheck);
el.openData.addEventListener('click', () => OpenDataFolder().catch((e) => appendLog(t('Không mở được thư mục: ', 'Could not open the folder: ') + errText(e))));
el.quit.addEventListener('click', () => { flushSave().catch(() => {}).finally(() => Quit()); });

el.modeInputs.forEach((i) => i.addEventListener('change', onFormChange));
el.langSelect.addEventListener('change', () => {
    onFormChange();
    flushSave().catch(() => {}); // apply now: the backend answers with the new language
});
[el.lan, el.rDirectVN, el.killSwitch, el.autoReconnect, el.autoConnect,
    el.startWin, el.startMin, el.closeTray, el.notify, el.interruptSwitch].forEach((i) => i.addEventListener('change', onFormChange));
[el.doh, el.rDirectDomains, el.rProxyDomains, el.rDirectProcs]
    .forEach((i) => i.addEventListener('input', onFormChange));

// "?" buttons expand the explanation whose id is in data-help. Capture phase +
// preventDefault so a button next to / inside a <label> never toggles the checkbox.
document.addEventListener('click', (ev) => {
    const b = ev.target.closest('button.help');
    if (!b) return;
    ev.preventDefault();
    ev.stopPropagation();
    const box = document.getElementById(b.dataset.help);
    if (!box) return;
    box.hidden = !box.hidden;
    b.setAttribute('aria-expanded', String(!box.hidden));
    b.classList.toggle('open', !box.hidden);
}, true);

EventsOn('log', appendLog);
EventsOn('status', applyStatus);
EventsOn('stats', applyStats);

// The VPS tab adds its users' links to the server list through the same action
// as the "Thêm server" box, so settings stay in sync.
initVPS({
    getServers: () => settings?.servers || [],
    importLinks: async (text) => {
        const before = settings?.servers?.length || 0;
        const s = await runAction('add', () => AddServers(text));
        if ((s?.servers?.length || 0) <= before) throw new Error(t('Link này đã có trong app.', 'This link is already in the app.'));
    },
});

async function init() {
    let tab = 'conn';
    try { tab = localStorage.getItem('rcv-tab') || 'conn'; } catch { /* ignore */ }
    showTab(tab);
    render();
    try {
        // Lines logged before the UI subscribed (startup, auto-connect).
        const past = await GetLogs();
        el.log.textContent = '';
        logLines = 0;
        (past || []).forEach(appendLog);
    } catch (e) { console.error('GetLogs', e); }
    try {
        settings = normalize(await GetSettings());
        fillForm(settings);
    } catch (e) {
        callError = t('Không đọc được cấu hình: ', 'Could not read the settings: ') + errText(e);
    }
    await pollStatus();
    render();
    setInterval(pollStatus, POLL_MS);
    setInterval(renderDuration, 1000);
}

init();

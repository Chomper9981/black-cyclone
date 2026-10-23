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
    // bypass
    bypassPanel: $('bypass-panel'),
    dpi: $('dpi'),
    dpiOpts: $('dpi-opts'),
    dpiPreset: $('dpi-preset'),
    dpiArgsRow: $('dpi-args-row'),
    dpiArgs: $('dpi-args'),
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
    routingBlock: $('routing-block'),
    routingNote: $('routing-note'),
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
    if (e == null) return 'Lỗi không xác định';
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
    if (!s.mode) s.mode = 'bypass';
    if (!s.dpi_preset) s.dpi_preset = '9';
    if (!s.selected_server) s.selected_server = 'auto';
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
    const t = Date.parse(iso || '');
    if (!t) return 'chưa cập nhật';
    const s = Math.max(0, Math.floor((Date.now() - t) / 1000));
    if (s < 60) return 'vừa xong';
    if (s < 3600) return `${Math.floor(s / 60)} phút trước`;
    if (s < 86400) return `${Math.floor(s / 3600)} giờ trước`;
    return `${Math.floor(s / 86400)} ngày trước`;
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
    el.dpi.checked = !!s.use_dpi;
    el.dpiPreset.value = s.dpi_preset || '9';
    if (el.dpiPreset.selectedIndex < 0) el.dpiPreset.value = '9';
    el.dpiArgs.value = s.dpi_args || '';
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
        mode: checked ? checked.value : (settings?.mode || 'bypass'),
        use_dpi: el.dpi.checked,
        dpi_preset: el.dpiPreset.value,
        dpi_args: el.dpiArgs.value.trim(),
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
        .catch((e) => { setMsg(el.saveMsg, 'Không lưu được: ' + errText(e), true); throw e; });
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
    if (!status.admin) parts.push('<p>Cần chạy với quyền Administrator.</p>');
    if (!status.bins_ok) {
        const missing = (status.missing || []).map((m) => `<li><code>${escapeHtml(m)}</code></li>`).join('');
        parts.push(`<p>Thiếu file cần thiết${missing ? ':' : '.'}</p>${missing ? `<ul>${missing}</ul>` : ''}` +
            '<p>Hãy chạy <code>scripts\\fetch-bins.ps1</code> rồi mở lại ứng dụng.</p>');
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
        on: ['on', 'Đã kết nối'],
        reconnecting: ['warn', 'Đang kết nối lại…'],
        blocked: ['err', 'Đang chặn mạng'],
    }[state] || (errMsg ? ['err', 'Lỗi'] : ['off', 'Chưa kết nối']);
    el.pill.classList.add(pill[0]);
    el.pill.textContent = pill[1];

    el.toggle.classList.toggle('on', state === 'on');
    el.toggle.classList.toggle('warn', state === 'reconnecting');
    el.toggle.classList.toggle('blocked', state === 'blocked');
    el.toggle.classList.toggle('busy', busy || state === 'reconnecting');
    el.toggle.disabled = busy || !settings;
    el.toggleLabel.textContent = busy
        ? (status.running ? 'Đang ngắt…' : 'Đang kết nối…')
        : (status.running ? 'Ngắt kết nối' : 'Kết nối');

    el.error.hidden = !errMsg;
    el.error.textContent = errMsg;

    // Kill switch banner
    const blocking = status.running && (state === 'blocked' || status.blocking);
    el.blocked.hidden = !blocking;
    if (blocking) {
        el.blocked.classList.toggle('amber', state === 'reconnecting');
        el.blockedTitle.textContent = state === 'reconnecting'
            ? 'Đang kết nối lại — kill switch tạm chặn mạng'
            : 'Kill switch đang chặn mạng';
        el.blockedText.textContent = state === 'reconnecting'
            ? 'VPN bị rớt. Internet bị chặn để không lộ IP thật cho tới khi kết nối lại được.'
            : 'VPN bị rớt và không kết nối lại được. Mọi kết nối Internet đang bị chặn để không lộ IP thật. Bấm “Kết nối lại” hoặc ngắt để dùng mạng bình thường.';
        el.blockedRelease.disabled = busy;
        el.blockedRetry.disabled = busy;
        el.blockedRetry.hidden = state === 'reconnecting';
    }

    // Pending
    el.pending.hidden = !(state === 'on' && status.pending);
    el.pendingReconnect.disabled = busy;
    el.pendingReconnect.textContent = busy ? 'Đang kết nối lại…' : 'Kết nối lại';

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
    if (state === 'reconnecting') { el.duration.textContent = 'Mất kết nối, đang thử lại…'; return; }
    if (state === 'blocked') { el.duration.textContent = 'Mất kết nối'; return; }
    if (state !== 'on') { el.duration.innerHTML = '&nbsp;'; return; }
    const since = status.since ? Date.parse(status.since) : NaN;
    if (!since || Number.isNaN(since)) { el.duration.textContent = 'Đang chạy'; return; }
    let s = Math.max(0, Math.floor((Date.now() - since) / 1000));
    const h = Math.floor(s / 3600); s -= h * 3600;
    const m = Math.floor(s / 60); s -= m * 60;
    const pad = (n) => String(n).padStart(2, '0');
    el.duration.textContent = `Đã kết nối ${h > 0 ? h + ':' : ''}${pad(m)}:${pad(s)}`;
}

function renderModes() {
    const mode = settings?.mode || 'bypass';
    document.querySelectorAll('.mode').forEach((card) => {
        card.classList.toggle('sel', card.dataset.mode === mode);
    });
    el.bypassPanel.hidden = mode !== 'bypass';
    el.warpPanel.hidden = mode !== 'warp';
    el.customPanel.hidden = mode !== 'custom';
    el.subPanel.hidden = mode !== 'custom';

    // bypass
    el.dpiOpts.classList.toggle('dim', !el.dpi.checked);
    el.dpiArgsRow.hidden = el.dpiPreset.value !== 'custom';
}

function warpTypeLabel(t) {
    switch (t) {
    case 'unlimited': return ['WARP+', 'plus'];
    case 'limited': return ['WARP+ (giới hạn)', 'plus'];
    case 'free': case '': case undefined: return ['Miễn phí', ''];
    default: return [t, ''];
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
    el.warpRegister.textContent = regBusy ? 'Đang đăng ký…' : 'Đăng ký WARP';
    el.warpReregister.disabled = regBusy;
    el.warpReregister.textContent = regBusy ? 'đang đăng ký…'
        : armed.has('rereg') ? 'nhấn lần nữa để xác nhận' : 'đăng ký lại';
    if (!has) return;

    const [label, cls] = warpTypeLabel(w.account_type);
    el.warpType.textContent = label;
    el.warpType.className = 'badge ' + cls;

    const licBusy = isBusy('license');
    el.warpKeyApply.disabled = licBusy || !el.warpKey.value.trim();
    el.warpKeyApply.textContent = licBusy ? 'Đang áp dụng…' : 'Áp dụng';

    el.warpEndpoint.textContent = w.endpoint ? hostPort(w.endpoint, w.endpoint_port) : 'Mặc định';

    const tunneled = status.running && status.mode !== 'bypass';
    const scanBusy = isBusy('scan');
    el.warpScan.disabled = tunneled || scanBusy;
    el.warpScan.textContent = scanBusy ? 'Đang quét… (~30s)' : 'Quét endpoint';
    el.warpScanHint.textContent = tunneled ? 'Ngắt kết nối trước khi quét' : '';

    el.warpScanList.hidden = !scanResults;
    if (scanResults) {
        if (scanResults.length === 0) {
            setHTML(el.warpScanList, '<div class="empty">Không có endpoint nào phản hồi.</div>');
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
    if (ms === -1) return '<span class="ms bad">lỗi</span>';
    if (ms === -2) return '<span class="ms na" title="Không đo được giao thức UDP khi chưa kết nối">—</span>';
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
        '<span class="srv-main"><span class="srv-name">Tự động (nhanh nhất)</span>' +
        '<span class="srv-meta">Chọn server có độ trễ thấp nhất</span></span></label>');
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
                `<span class="srv-name" title="${escapeHtml(s.name)}">${escapeHtml(s.name || '(không tên)')}</span>` +
                `<span class="srv-meta"><span class="proto">${escapeHtml(protoOf(s.link))}</span>` +
                (from ? `<span class="from">${escapeHtml(from)}</span>` : '') + '</span></span>';
        }
        rows.push(
            `<label class="srv${isSel ? ' sel' : ''}">` +
            `<input type="radio" name="srv" value="${id}"${isSel ? ' checked' : ''}>` +
            main +
            pingBadge(s.id) +
            '<span class="srv-acts">' +
            `<button type="button" class="icon" data-act="rename" data-id="${id}" title="Đổi tên" aria-label="Đổi tên">✎</button>` +
            `<button type="button" class="icon del${delArmed ? ' armed' : ''}" data-act="del" data-id="${id}" ` +
            `title="${delArmed ? 'Nhấn lần nữa để xoá' : 'Xoá'}" aria-label="Xoá">${delArmed ? 'xoá?' : '✕'}</button>` +
            '</span></label>');
    }
    if (settings.servers.length === 0) {
        rows.push('<div class="empty">Chưa có server. Thêm link hoặc subscription bên dưới.</div>');
    }
    const changed = setHTML(el.srvList, rows.join(''));
    if (renaming && changed) {
        const inp = el.srvList.querySelector('.srv-rename');
        if (inp) { inp.focus(); inp.select(); }
    }

    const pingBusy = isBusy('ping');
    el.srvPing.disabled = pingBusy || settings.servers.length === 0;
    el.srvPing.textContent = pingBusy ? 'đang ping…' : 'ping';
    const addBusy = isBusy('add');
    el.srvAddBtn.disabled = addBusy;
    el.srvAddBtn.textContent = addBusy ? 'Đang thêm…' : 'Thêm';
}

function renderSubs() {
    if (!settings || settings.mode !== 'custom') return;
    const counts = new Map();
    for (const s of settings.servers) if (s.sub_id) counts.set(s.sub_id, (counts.get(s.sub_id) || 0) + 1);
    const subs = settings.subscriptions;
    setHTML(el.subList, subs.length === 0
        ? '<div class="empty">Chưa có subscription.</div>'
        : subs.map((s) => {
            const delArmed = armed.has('sub:' + s.id);
            return '<div class="sub-row">' +
                `<span class="srv-main"><span class="srv-name" title="${escapeHtml(s.url)}">${escapeHtml(s.name)}</span>` +
                `<span class="srv-meta">${counts.get(s.id) || 0} server · ${escapeHtml(fmtAgo(s.updated))}</span></span>` +
                `<button type="button" class="icon del${delArmed ? ' armed' : ''}" data-act="sub-del" data-id="${escapeHtml(s.id)}" ` +
                `title="${delArmed ? 'Nhấn lần nữa để xoá' : 'Xoá subscription và server của nó'}" aria-label="Xoá">${delArmed ? 'xoá?' : '✕'}</button>` +
                '</div>';
        }).join(''));
    const refBusy = isBusy('sub-refresh');
    el.subRefresh.disabled = refBusy || subs.length === 0;
    el.subRefresh.textContent = refBusy ? 'đang cập nhật…' : 'cập nhật tất cả';
    const addBusy = isBusy('sub-add');
    el.subAddBtn.disabled = addBusy;
    el.subAddBtn.textContent = addBusy ? 'Đang tải…' : 'Thêm';
}

function renderOptions() {
    const mode = settings?.mode || 'bypass';
    const routed = mode !== 'bypass';
    el.routingBlock.classList.toggle('dim', !routed);
    el.routingNote.hidden = routed;
    el.startMinRow.classList.toggle('dim', !el.startWin.checked);
    el.startMin.disabled = !el.startWin.checked;

    const checkBusy = isBusy('check');
    el.checkBtn.disabled = checkBusy;
    el.checkBtn.textContent = checkBusy ? 'Đang kiểm tra…' : 'Kiểm tra kết nối';
}

function render() {
    renderHero();
    renderModes();
    renderWarp();
    renderServers();
    renderSubs();
    renderOptions();
    renderWarnings();
}

/* ---------------- Status & stats ---------------- */

function applyStatus(st) {
    if (!st) return;
    status = {...status, ...st, missing: st.missing || [], warnings: st.warnings || []};
    if (connState() !== 'on') stats = null;
    render();
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
            setMsg(el.warpMsg, 'Đăng ký WARP thành công.' + (status.running && status.mode === 'warp' ? ' Kết nối lại để dùng tài khoản mới.' : ''));
            scanResults = null;
        } else {
            setMsg(el.warpMsg, 'Đăng ký thất bại — xem nhật ký để biết chi tiết.', true);
        }
    } catch (e) {
        setMsg(el.warpMsg, 'Đăng ký thất bại: ' + errText(e), true);
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
        setMsg(el.warpMsg, 'Đã áp dụng key. Loại tài khoản: ' + warpTypeLabel(s?.warp?.account_type)[0] + '.');
    } catch (e) {
        setMsg(el.warpMsg, 'Không áp dụng được key: ' + errText(e), true);
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
        setMsg(el.warpMsg, 'Quét thất bại: ' + errText(e), true);
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
        setMsg(el.warpMsg, `Đã chọn endpoint ${hostPort(r.ip, r.port)}.`);
    } catch (e) {
        setMsg(el.warpMsg, 'Không đặt được endpoint: ' + errText(e), true);
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
            .catch((e) => setMsg(el.srvMsg, 'Không xoá được: ' + errText(e), true));
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
        setMsg(el.srvMsg, 'Không đổi tên được: ' + errText(e), true);
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
            setMsg(el.srvMsg, `Đã thêm ${added} server.`);
        } else {
            setMsg(el.srvMsg, 'Không thêm được server nào (link không hỗ trợ hoặc đã có).', true);
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
        setMsg(el.srvMsg, 'Ping thất bại: ' + errText(e), true);
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
        setMsg(el.subMsg, 'URL phải bắt đầu bằng http:// hoặc https://', true);
        return;
    }
    setMsg(el.subMsg, '');
    try {
        await runAction('sub-add', () => AddSubscription(url, el.subName.value.trim()));
        el.subUrl.value = '';
        el.subName.value = '';
        setMsg(el.subMsg, 'Đã thêm subscription.');
    } catch (e) {
        setMsg(el.subMsg, 'Không tải được subscription: ' + errText(e), true);
    }
    render();
}

async function onRefreshSubs() {
    if (isBusy('sub-refresh')) return;
    setMsg(el.subMsg, '');
    try {
        await runAction('sub-refresh', RefreshSubscriptions);
        setMsg(el.subMsg, 'Đã cập nhật.');
    } catch (e) {
        setMsg(el.subMsg, 'Cập nhật lỗi: ' + errText(e), true);
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
        .catch((e) => setMsg(el.subMsg, 'Không xoá được: ' + errText(e), true));
}

/* ---------------- Tools ---------------- */

const WARP_FLAG = {on: 'WARP', plus: 'WARP+', off: 'không'};

async function onCheck() {
    if (isBusy('check')) return;
    ops.add('check');
    render();
    try {
        const r = await CheckConnection();
        const ip = r?.ip || {};
        const head = ip.error
            ? `<div class="ip-err">Không lấy được IP: ${escapeHtml(ip.error)}</div>`
            : '<div class="ip-grid">' +
              `<span>IP</span><code>${escapeHtml(ip.ip || '?')}</code>` +
              `<span>Quốc gia</span><b>${escapeHtml(ip.country || '?')}</b>` +
              `<span>Cloudflare</span><b>${escapeHtml(ip.colo || '?')}</b>` +
              `<span>WARP</span><b>${escapeHtml(WARP_FLAG[ip.warp] || ip.warp || '?')}</b></div>`;
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
el.openData.addEventListener('click', () => OpenDataFolder().catch((e) => appendLog('Không mở được thư mục: ' + errText(e))));
el.quit.addEventListener('click', () => { flushSave().catch(() => {}).finally(() => Quit()); });

el.modeInputs.forEach((i) => i.addEventListener('change', onFormChange));
[el.dpi, el.dpiPreset, el.lan, el.rDirectVN, el.killSwitch, el.autoReconnect, el.autoConnect,
    el.startWin, el.startMin, el.closeTray, el.notify, el.interruptSwitch].forEach((i) => i.addEventListener('change', onFormChange));
[el.dpiArgs, el.doh, el.rDirectDomains, el.rProxyDomains, el.rDirectProcs]
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
        callError = 'Không đọc được cấu hình: ' + errText(e);
    }
    await pollStatus();
    render();
    setInterval(pollStatus, POLL_MS);
    setInterval(renderDuration, 1000);
}

init();

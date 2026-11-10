// Tab "VPS": manage the user's own Black Cyclone servers over SSH.
import {
    ListVPS,
    AddVPS,
    UpdateVPS,
    TrustNewVPSKey,
    RemoveVPS,
    PickKeyFile,
    VPSOverview,
    VPSInstall,
    VPSAddUser,
    VPSRemoveUser,
    VPSRestart,
    VPSLogs,
    VPSUninstall,
    VPSLinkQR,
    VPSResetTraffic,
} from '../wailsjs/go/main/App';
import {EventsOn, ClipboardSetText} from '../wailsjs/runtime/runtime';
import {t} from './i18n.js';

const $ = (id) => document.getElementById(id);
const ARM_MS = 3000;
const OUT_MAX = 400;
const POLL_MS = 20000;   // refresh who is online while the tab is open

const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[c]));
const errText = (e) => {
    const msg = e == null ? t('Lỗi không xác định', 'Unknown error') : typeof e === 'string' ? e : (e.message || String(e));
    return msg.charAt(0).toUpperCase() + msg.slice(1); // Go errors start lowercase
};
function fmtBytes(n) {
    n = Math.max(0, Number(n) || 0);
    if (n < 1024) return `${n} B`;
    const units = ['KB', 'MB', 'GB', 'TB'];
    let i = -1;
    do { n /= 1024; i++; } while (n >= 1024 && i < units.length - 1);
    return `${n < 10 ? n.toFixed(1) : Math.round(n)} ${units[i]}`;
}

// A link used from this many addresses at once has probably been shared.
const MANY_IPS = 3;

const hostPort = (h, p) => (String(h).includes(':') ? `[${h}]` : h) + (p ? `:${p}` : '');

let hosts = [];
let selected = null;
const overviews = new Map();   // id -> {state: 'loading'|'ok'|'error', data, error}
let op = '';                   // operation in flight on the selected VPS
let adding = false;
let editing = null;             // id of the VPS whose details the form is editing
const armed = new Map();
let outLines = [];
let outFor = null;
let qr = null;                  // {id, name, src} QR shown under a user
let app = null;                // hooks from main.js

const el = {};

function setMsg(node, text, bad) {
    node.hidden = !text;
    node.textContent = text || '';
    node.classList.toggle('bad', !!bad);
}

function arm(key) {
    disarm(key);
    armed.set(key, setTimeout(() => { armed.delete(key); render(); }, ARM_MS));
    render();
}
function disarm(key) {
    if (armed.has(key)) clearTimeout(armed.get(key));
    armed.delete(key);
}

/* ---------------- Rendering ---------------- */

function renderList() {
    if (hosts.length === 0) {
        el.list.innerHTML = `<div class="empty">${t('Chưa có VPS. Thêm VPS bên dưới để cài và quản lý server riêng.', 'No VPS yet. Add one below to install and manage your own server.')}</div>`;
        return;
    }
    el.list.innerHTML = hosts.map((h) => {
        const sel = h.id === selected;
        const delArmed = armed.has('host:' + h.id);
        return `<label class="srv${sel ? ' sel' : ''}">` +
            `<input type="radio" name="vps" value="${esc(h.id)}"${sel ? ' checked' : ''}>` +
            `<span class="srv-main"><span class="srv-name">${esc(h.name)}</span>` +
            `<span class="srv-meta">${esc(h.user)}@${esc(hostPort(h.addr, h.port))} · ${h.auth === 'key' ? t('khoá SSH', 'SSH key') : t('mật khẩu', 'password')}</span></span>` +
            '<span class="srv-acts">' +
            `<button type="button" class="icon" data-act="host-edit" data-id="${esc(h.id)}" title="${t('Sửa thông tin đăng nhập', 'Edit the login details')}" aria-label="${t('Sửa', 'Edit')}">✎</button>` +
            `<button type="button" class="icon del${delArmed ? ' armed' : ''}" data-act="host-del" data-id="${esc(h.id)}" ` +
            `title="${delArmed ? t('Nhấn lần nữa để xoá', 'Click again to remove') : t('Xoá khỏi app (server trên VPS vẫn chạy)', 'Remove from the app (the server on the VPS keeps running)')}" aria-label="${t('Xoá', 'Remove')}">${delArmed ? t('xoá?', 'remove?') : '✕'}</button>` +
            '</span></label>';
    }).join('');
}

function stateBadge(active) {
    if (active === 'active') return `<span class="state on">${t('Đang chạy', 'Running')}</span>`;
    if (active === 'activating' || active === 'reloading') return `<span class="state warn">${t('Đang khởi động', 'Starting')}</span>`;
    return `<span class="state off">${esc(active === 'failed' ? t('Lỗi', 'Failed') : t('Đã dừng', 'Stopped'))}</span>`;
}

function userMeta(o, u, inApp) {
    const parts = [];
    if (u.conns > 0) {
        const ips = u.ips || [];
        const many = ips.length >= MANY_IPS;
        parts.push(`<span class="online${many ? ' many' : ''}" title="${esc(ips.join(', '))}">● online · ${ips.length} IP` +
            `${many ? t(' (link bị chia sẻ?)', ' (link shared?)') : ''}</span>`);
    }
    if (o.stats) parts.push(`↓ ${esc(fmtBytes(u.down))} ↑ ${esc(fmtBytes(u.up))}`);
    if (inApp) parts.push(`<span class="in-app">${t('đã có trong app', 'in the app')}</span>`);
    if (parts.length === 0) parts.push(t('link riêng của người này', 'their own link'));
    return parts.join('<span class="sep">·</span>');
}

function renderInstallForm(update) {
    const b = op === 'install';
    const keep = t('giữ nguyên', 'unchanged');
    return '<div class="vps-install">' +
        (update ? '' : `<p class="hint">${t('Chưa cài server trên VPS này. App sẽ cài sing-box (VLESS + Reality), bật BBR, mở tường lửa và tạo người dùng đầu tiên. Mất khoảng 1–2 phút.', 'No server on this VPS yet. The app will install sing-box (VLESS + Reality), enable BBR, open the firewall and create the first user. It takes 1–2 minutes.')}</p>` +
            `<label class="field"><span>${t('Tên người dùng đầu tiên', 'First user name')}</span>` +
            `<input id="vi-name" type="text" spellcheck="false" placeholder="${t('vd. laptop-cua-toi (mặc định: default)', 'e.g. my-laptop (default: default)')}"></label>`) +
        `<details class="inner"><summary>${t('Tuỳ chọn nâng cao', 'Advanced options')}</summary>` +
        '<div class="field-row">' +
        `<label class="field port"><span>${t('Cổng', 'Port')}</span><input id="vi-port" type="number" min="1" max="65535" placeholder="${update ? keep : '443'}"></label>` +
        `<label class="field grow"><span>${t('Trang giả làm (SNI)', 'Site to imitate (SNI)')}</span><input id="vi-sni" type="text" spellcheck="false" placeholder="${update ? keep : 'www.microsoft.com'}"></label>` +
        '</div>' +
        `<label class="field"><span>${t('Địa chỉ ghi vào link', 'Address written into links')}</span><input id="vi-host" type="text" spellcheck="false" placeholder="${update ? keep : t('để trống: IP công khai của VPS', "empty: the VPS's public IP")}"></label>` +
        `<p class="hint">${t('Đổi cổng, SNI hoặc địa chỉ thì link cũ của mọi người hết dùng được: gửi lại link mới cho họ.', "Changing the port, SNI or address invalidates everyone's old links: send them the new ones.")}</p>` +
        '</details>' +
        '<div class="row end">' +
        (update ? `<button type="button" class="link" data-act="install-cancel">${t('huỷ', 'cancel')}</button>` : '') +
        `<button type="button" class="btn sm" data-act="install"${b || op ? ' disabled' : ''}>${b ? t('Đang cài…', 'Installing…') : update ? t('Cập nhật server', 'Update server') : t('Cài server', 'Install server')}</button>` +
        '</div></div>';
}

let showUpdate = false;

function renderDetail() {
    const h = hosts.find((x) => x.id === selected);
    el.detail.hidden = !h;
    if (!h) return;
    el.title.textContent = h.name;
    el.refresh.disabled = !!op;
    el.refresh.textContent = op === 'load' ? t('đang tải…', 'loading…') : t('làm mới', 'refresh');

    const ov = overviews.get(h.id);
    let html = '';
    if (!ov || ov.state === 'loading') {
        html = `<div class="empty">${t('Đang kết nối tới VPS…', 'Connecting to the VPS…')}</div>`;
    } else if (ov.state === 'error') {
        html = `<div class="ip-err">${esc(ov.error)}</div>`;
        if (/khoá SSH của VPS đã thay đổi|SSH key has changed/i.test(ov.error)) {
            const trustArmed = armed.has('trust');
            html += `<p class="hint">${t('Chỉ chấp nhận khoá mới nếu chính bạn vừa cài lại hệ điều hành VPS hoặc nhà cung cấp vừa đổi máy.', "Accept the new key only if you have just reinstalled the VPS's operating system or your provider moved it to another machine.")}</p>` +
                `<div class="row"><button type="button" class="btn ghost sm danger" data-act="trust"${op ? ' disabled' : ''}>` +
                `${op === 'trust' ? t('Đang kết nối…', 'Connecting…') : trustArmed ? t('Nhấn lần nữa: tôi vừa cài lại VPS', 'Click again: I have just reinstalled the VPS') : t('Chấp nhận khoá mới', 'Accept the new key')}</button></div>`;
        }
    } else if (!ov.data.installed) {
        html = renderInstallForm(false);
    } else {
        const o = ov.data;
        const inApp = new Set((app.getServers() || []).map((s) => s.link));
        html += '<div class="ip-grid vps-grid">' +
            `<span>${t('Trạng thái', 'State')}</span>${stateBadge(o.active)}` +
            `<span>sing-box</span><b>${esc(o.version || '?')}</b>` +
            `<span>${t('Địa chỉ', 'Address')}</span><code class="wide">${esc(hostPort(o.host, o.port))}</code>` +
            `<span>SNI</span><code class="wide">${esc(o.sni)}</code>` +
            '</div>';
        if (o.script_outdated && !showUpdate) {
            html += `<div class="vps-note">${t('Bản server trên VPS cũ hơn bản đi kèm app này.', 'The server on the VPS is older than the one shipped with this app.')} ` +
                `<button type="button" class="link" data-act="show-update"${op ? ' disabled' : ''}>${t('cập nhật', 'update')}</button></div>`;
        }

        let total = '';
        if (o.stats) {
            const up = o.users.reduce((a, u) => a + (u.up || 0), 0);
            const down = o.users.reduce((a, u) => a + (u.down || 0), 0);
            const rArmed = armed.has('reset');
            total = `<span class="vps-total">${t('tổng', 'total')} ↓ ${esc(fmtBytes(down))} ↑ ${esc(fmtBytes(up))}</span>` +
                `<button type="button" class="link" data-act="reset"${op ? ' disabled' : ''} title="${t('Đặt lại thống kê lưu lượng của mọi người về 0', "Reset everyone's traffic counters to 0")}">` +
                `${op === 'reset' ? t('đang đặt lại…', 'resetting…') : rArmed ? t('nhấn lần nữa để đặt lại', 'click again to reset') : t('đặt lại', 'reset')}</button>`;
        }
        const online = o.users.filter((u) => u.conns > 0).length;
        html += `<div class="vps-sub"><span>${t('Người dùng', 'Users')} <span class="count">(` + o.users.length +
            (o.online ? t(`, ${online} đang online`, `, ${online} online`) : '') + ')</span></span>' + total + '</div><div class="sub-list">';
        for (const u of o.users || []) {
            const delArmed = armed.has('user:' + u.name);
            const has = inApp.has(u.link);
            const last = o.users.length <= 1;
            html += '<div class="sub-row">' +
                `<span class="srv-main"><span class="srv-name">${esc(u.name)}</span>` +
                `<span class="srv-meta">${userMeta(o, u, has)}</span></span>` +
                '<span class="srv-acts">' +
                `<button type="button" class="icon${qr && qr.id === h.id && qr.name === u.name ? ' on' : ''}" data-act="qr" data-name="${esc(u.name)}" title="${t('Hiện mã QR để điện thoại quét', 'Show a QR code for a phone to scan')}">QR</button>` +
                `<button type="button" class="icon" data-act="copy" data-name="${esc(u.name)}" title="${t('Copy link để gửi cho người này', 'Copy the link to send to this user')}">copy</button>` +
                (has ? '' : `<button type="button" class="icon" data-act="import" data-name="${esc(u.name)}" title="${t('Thêm link này vào danh sách server của app trên máy này', "Add this link to this computer's server list")}">+ app</button>`) +
                (last ? '' : `<button type="button" class="icon del${delArmed ? ' armed' : ''}" data-act="user-del" data-name="${esc(u.name)}" ` +
                    `title="${delArmed ? t('Nhấn lần nữa để xoá', 'Click again to remove') : t('Xoá người dùng: link của họ hết dùng được ngay', 'Remove the user: their link stops working at once')}" aria-label="${t('Xoá', 'Remove')}"${op ? ' disabled' : ''}>${delArmed ? t('xoá?', 'remove?') : '✕'}</button>`) +
                '</span></div>';
            if (qr && qr.id === h.id && qr.name === u.name) {
                html += '<div class="qr-box">' +
                    `<img src="${esc(qr.src)}" alt="${t('Mã QR link của', 'QR code of the link of')} ${esc(u.name)}" width="220" height="220">` +
                    '<p class="hint">' + t(`Trên điện thoại của <b>${esc(u.name)}</b>: quét bằng camera hoặc Google Lens, copy link rồi dán vào Black Cyclone → Server riêng. ` +
                    'Mã QR chứa link đầy đủ: ai chụp được nó cũng dùng được server.',
                    `On <b>${esc(u.name)}</b>'s phone: scan with the camera or Google Lens, copy the link and paste it into Black Cyclone → Own server. ` +
                    'The QR code holds the whole link: anyone who photographs it can use the server.') + '</p></div>';
            }
        }
        html += '</div>';
        html += '<div class="row">' +
            `<input id="vu-name" type="text" spellcheck="false" placeholder="${t('Tên người dùng mới, vd. dien-thoai-me', 'New user name, e.g. moms-phone')}">` +
            `<button type="button" class="btn sm" data-act="user-add"${op ? ' disabled' : ''}>${op === 'user-add' ? t('Đang thêm…', 'Adding…') : t('Thêm', 'Add')}</button></div>` +
            `<p class="hint">${t('Mỗi người / thiết bị một tên. Muốn cắt quyền ai thì chỉ cần xoá đúng tên đó.', "One name per person / device. To cut someone off, remove just their name.")}</p>`;

        if (showUpdate) html += renderInstallForm(true);

        const unArmed = armed.has('uninstall');
        html += '<div class="vps-actions">' +
            `<button type="button" class="btn ghost sm" data-act="restart"${op ? ' disabled' : ''}>${op === 'restart' ? t('Đang khởi động lại…', 'Restarting…') : t('Khởi động lại', 'Restart')}</button>` +
            `<button type="button" class="btn ghost sm" data-act="logs"${op ? ' disabled' : ''}>${op === 'logs' ? t('Đang tải…', 'Loading…') : t('Xem log', 'View log')}</button>` +
            (showUpdate ? '' : `<button type="button" class="btn ghost sm" data-act="show-update"${op ? ' disabled' : ''}>${t('Cập nhật / đổi cấu hình', 'Update / change settings')}</button>`) +
            `<button type="button" class="btn ghost sm danger" data-act="uninstall"${op ? ' disabled' : ''}>` +
            `${op === 'uninstall' ? t('Đang gỡ…', 'Removing…') : unArmed ? t('Nhấn lần nữa: gỡ hết, mọi link mất hiệu lực', 'Click again: removes everything, all links stop working') : t('Gỡ server', 'Remove server')}</button>` +
            '</div>';
    }
    html += `<div class="vps-fp">${t('Khoá SSH', 'SSH key')}: <code>${esc(h.fingerprint || '?')}</code></div>`;

    // Keep what the user is typing across re-renders.
    const keep = {};
    for (const id of ['vu-name', 'vi-name', 'vi-port', 'vi-sni', 'vi-host']) {
        const n = $(id);
        if (n) keep[id] = n.value;
    }
    const focused = document.activeElement && document.activeElement.id;
    const openDetails = !!el.body.querySelector('.vps-install details[open]');
    if (el.body._html !== html) {
        el.body._html = html;
        el.body.innerHTML = html;
        for (const [id, v] of Object.entries(keep)) { const n = $(id); if (n) n.value = v; }
        if (openDetails) { const d = el.body.querySelector('.vps-install details'); if (d) d.open = true; }
        if (focused) { const n = $(focused); if (n && el.body.contains(n)) n.focus(); }
    }

    el.out.hidden = outFor !== h.id || outLines.length === 0;
    const text = outLines.join('\n');
    if (!el.out.hidden && el.out.textContent !== text) { // don't yank the scroll on every status poll
        el.out.textContent = text;
        el.out.scrollTop = el.out.scrollHeight;
    }
}

function renderForm() {
    const key = el.form.querySelector('input[name="vps-auth"]:checked')?.value === 'key';
    el.keyRow.hidden = !key;
    el.passLabel.textContent = key ? t('Mật khẩu (của khoá hoặc sudo, nếu có)', 'Password (of the key or sudo, if any)') : t('Mật khẩu', 'Password');
    el.addBtn.disabled = adding;
    el.addBtn.textContent = adding ? t('Đang kiểm tra…', 'Checking…') : editing ? t('Lưu thay đổi', 'Save changes') : t('Kiểm tra & lưu', 'Check & save');
    const h = editing && hosts.find((x) => x.id === editing);
    el.summary.textContent = h ? `${t('Sửa VPS', 'Edit VPS')}: ${h.name}` : t('Thêm VPS', 'Add a VPS');
    el.cancelEdit.hidden = !editing;
    el.pass.placeholder = editing ? t('để trống: giữ mật khẩu đã lưu', 'empty: keep the saved password') : '';
}

function fillForm(h) {
    el.name.value = h ? h.name : '';
    el.addr.value = h ? h.addr : '';
    el.port.value = h ? h.port : 22;
    el.user.value = h ? h.user : 'root';
    el.key.value = h ? (h.key_file || '') : '';
    el.pass.value = '';
    const auth = h && h.auth === 'key' ? 'key' : 'password';
    el.form.querySelector(`input[name="vps-auth"][value="${auth}"]`).checked = true;
}

function startEdit(id) {
    const h = hosts.find((x) => x.id === id);
    if (!h) return;
    editing = id;
    fillForm(h);
    el.addBox.open = true;
    setMsg(el.msg, '');
    render();
    el.addBox.scrollIntoView({block: 'nearest'});
}

function stopEdit() {
    editing = null;
    fillForm(null);
    el.addBox.open = false;
    render();
}

function render() {
    renderList();
    renderDetail();
    renderForm();
}

/* ---------------- Actions ---------------- */

async function loadHosts() {
    try {
        hosts = (await ListVPS()) || [];
    } catch (e) {
        hosts = [];
        setMsg(el.msg, errText(e), true);
    }
    if (!hosts.some((h) => h.id === selected)) {
        let saved = null;
        try { saved = localStorage.getItem('rcv-vps'); } catch { /* ignore */ }
        selected = hosts.some((h) => h.id === saved) ? saved : (hosts[0]?.id || null);
    }
    render();
    ensureLoaded();
}

const paneVisible = () => !document.querySelector('.pane[data-pane="vps"]')?.hidden;

// ensureLoaded fetches the selected VPS's state once the tab is visible and idle.
function ensureLoaded() {
    if (paneVisible() && selected && !op && !overviews.has(selected)) refresh();
}

function select(id) {
    if (selected === id) return;
    selected = id;
    showUpdate = false;
    try { localStorage.setItem('rcv-vps', id); } catch { /* ignore */ }
    setMsg(el.detailMsg, '');
    if (overviews.get(id)?.state === 'error') overviews.delete(id);
    render();
    ensureLoaded();
}

// run performs an operation on the selected VPS that returns a new Overview.
async function run(name, fn, okMsg) {
    const id = selected;
    if (!id || op) return;
    op = name;
    setMsg(el.detailMsg, '');
    render();
    try {
        const data = await fn(id);
        if (data !== undefined) overviews.set(id, {state: 'ok', data: normOverview(data)});
        if (okMsg) setMsg(el.detailMsg, okMsg);
        return data;
    } catch (e) {
        if (name === 'load') overviews.set(id, {state: 'error', error: errText(e)});
        else setMsg(el.detailMsg, errText(e), true);
    } finally {
        op = '';
        render();
        ensureLoaded(); // the selection may have changed while this ran
    }
}

function normOverview(o) {
    return {...o, users: o?.users || []};
}

function refresh() {
    if (!selected || op) return;
    if (!overviews.has(selected) || overviews.get(selected).state === 'error') {
        overviews.set(selected, {state: 'loading'});
    }
    return run('load', VPSOverview);
}

async function onAddHost() {
    if (adding) return;
    const auth = el.form.querySelector('input[name="vps-auth"]:checked')?.value;
    const input = {
        name: el.name.value.trim(),
        addr: el.addr.value.trim(),
        port: Number(el.port.value) || 22,
        user: el.user.value.trim() || 'root',
        password: el.pass.value,
        key_file: auth === 'key' ? el.key.value.trim() : '',
    };
    if (!input.addr) { setMsg(el.msg, t('Nhập IP hoặc tên miền của VPS.', "Enter the VPS's IP or domain."), true); return; }
    if (auth === 'key' && !input.key_file) { setMsg(el.msg, t('Chọn file khoá SSH.', 'Choose the SSH key file.'), true); return; }
    const old = editing && hosts.find((x) => x.id === editing);
    // Editing: an empty password keeps the saved one (unless switching to password login).
    if (auth !== 'key' && !input.password && !(old && old.auth === 'password')) {
        setMsg(el.msg, t('Nhập mật khẩu SSH.', 'Enter the SSH password.'), true);
        return;
    }
    adding = true;
    setMsg(el.msg, '');
    render();
    try {
        const id = editing;
        const v = id ? await UpdateVPS(id, input) : await AddVPS(input);
        editing = null;
        fillForm(null);
        el.addBox.open = false;
        setMsg(el.msg, id ? t(`Đã lưu ${v.name}.`, `Saved ${v.name}.`) : t(`Đã thêm ${v.name}. Khoá SSH của VPS: ${v.fingerprint}`, `Added ${v.name}. The VPS's SSH key: ${v.fingerprint}`));
        if (id) overviews.delete(id);
        await loadHosts();
        if (id && selected === id) ensureLoaded(); else select(v.id);
    } catch (e) {
        setMsg(el.msg, errText(e), true);
    } finally {
        adding = false;
        render();
    }
}

async function onListClick(ev) {
    const b = ev.target.closest('button[data-act]');
    if (!b) return;
    ev.preventDefault();
    ev.stopPropagation();
    const id = b.dataset.id;
    if (b.dataset.act === 'host-edit') { startEdit(id); return; }
    const key = 'host:' + id;
    if (!armed.has(key)) { arm(key); return; }
    disarm(key);
    try {
        await RemoveVPS(id);
        overviews.delete(id);
        if (selected === id) selected = null;
        if (editing === id) stopEdit();
        setMsg(el.msg, t('Đã xoá VPS khỏi app. Server trên VPS vẫn chạy, link của mọi người vẫn dùng được.', "Removed the VPS from the app. The server on it keeps running and everyone's links still work."));
        await loadHosts();
        if (selected) refresh();
    } catch (e) {
        setMsg(el.msg, errText(e), true);
    }
}

function currentUsers() {
    const ov = overviews.get(selected);
    return ov?.state === 'ok' ? ov.data.users : [];
}

function installOpts(update) {
    const v = (id) => ($(id)?.value || '').trim();
    return {
        port: Number(v('vi-port')) || 0,
        sni: v('vi-sni'),
        host: v('vi-host'),
        name: update ? '' : v('vi-name'),
    };
}

async function onDetailClick(ev) {
    const b = ev.target.closest('button[data-act]');
    if (!b || b.disabled) return;
    const act = b.dataset.act;
    const name = b.dataset.name;
    const user = currentUsers().find((u) => u.name === name);
    switch (act) {
    case 'qr':
        if (!user) return;
        if (qr && qr.id === selected && qr.name === name) { qr = null; render(); return; }
        try {
            qr = {id: selected, name, src: await VPSLinkQR(user.link)};
        } catch (e) {
            setMsg(el.detailMsg, t('Không tạo được mã QR: ', 'Could not create the QR code: ') + errText(e), true);
        }
        render();
        return;
    case 'copy':
        if (!user) return;
        try {
            await ClipboardSetText(user.link);
            setMsg(el.detailMsg, t(`Đã copy link của ${name}. Gửi riêng cho người đó, đừng đăng công khai.`, `Copied ${name}'s link. Send it to them privately; do not post it publicly.`));
        } catch (e) {
            setMsg(el.detailMsg, t('Không copy được: ', 'Could not copy: ') + errText(e), true);
        }
        return;
    case 'import':
        if (!user) return;
        try {
            await app.importLinks(user.link);
            setMsg(el.detailMsg, t(`Đã thêm link của ${name} vào tab Kết nối → Server riêng.`, `Added ${name}'s link to the Connect tab → Own server.`));
        } catch (e) {
            setMsg(el.detailMsg, errText(e), true);
        }
        render();
        return;
    case 'user-add': {
        const inp = $('vu-name');
        const n = (inp?.value || '').trim();
        if (!/^[A-Za-z0-9._-]{1,32}$/.test(n)) {
            setMsg(el.detailMsg, t('Tên chỉ gồm chữ không dấu, số, ".", "_", "-" (tối đa 32 ký tự).', 'Names may only contain letters, digits, ".", "_", "-" (32 characters at most).'), true);
            return;
        }
        const r = await run('user-add', (id) => VPSAddUser(id, n), t(`Đã thêm ${n}. Bấm "copy" để lấy link gửi cho họ.`, `Added ${n}. Click "copy" to get the link to send them.`));
        if (r && inp) { const fresh = $('vu-name'); if (fresh) fresh.value = ''; }
        return;
    }
    case 'user-del': {
        const key = 'user:' + name;
        if (!armed.has(key)) { arm(key); return; }
        disarm(key);
        if (qr && qr.name === name) qr = null;
        run('user-del', (id) => VPSRemoveUser(id, name), t(`Đã xoá ${name}: link của người này hết dùng được.`, `Removed ${name}: their link no longer works.`));
        return;
    }
    case 'trust': {
        if (!armed.has('trust')) { arm('trust'); return; }
        disarm('trust');
        run('trust', async (id) => {
            await TrustNewVPSKey(id);
            await loadHosts(); // shows the new fingerprint
            return VPSOverview(id);
        }, t('Đã chấp nhận khoá SSH mới của VPS.', "Accepted the VPS's new SSH key."));
        return;
    }
    case 'reset':
        if (!armed.has('reset')) { arm('reset'); return; }
        disarm('reset');
        run('reset', VPSResetTraffic, t('Đã đặt lại thống kê lưu lượng.', 'Traffic counters reset.'));
        return;
    case 'restart':
        run('restart', VPSRestart, t('Đã khởi động lại server.', 'Server restarted.'));
        return;
    case 'logs':
        run('logs', async (id) => { await showLogs(id); return undefined; });
        return;
    case 'show-update':
        showUpdate = true;
        render();
        return;
    case 'install-cancel':
        showUpdate = false;
        render();
        return;
    case 'install': {
        const ov = overviews.get(selected);
        const update = ov?.state === 'ok' && ov.data.installed;
        const opts = installOpts(update);
        if (opts.name && !/^[A-Za-z0-9._-]{1,32}$/.test(opts.name)) {
            setMsg(el.detailMsg, t('Tên chỉ gồm chữ không dấu, số, ".", "_", "-" (tối đa 32 ký tự).', 'Names may only contain letters, digits, ".", "_", "-" (32 characters at most).'), true);
            return;
        }
        outFor = selected;
        outLines = [];
        const r = await run('install', (id) => VPSInstall(id, opts), update ? t('Đã cập nhật server.', 'Server updated.') : t('Đã cài server. Thêm link vào app bằng nút "+ app".', 'Server installed. Add the link to the app with the "+ app" button.'));
        if (r) showUpdate = false;
        render();
        return;
    }
    case 'uninstall':
        if (!armed.has('uninstall')) { arm('uninstall'); return; }
        disarm('uninstall');
        showUpdate = false;
        run('uninstall', VPSUninstall, t('Đã gỡ server khỏi VPS.', 'Server removed from the VPS.'));
        return;
    }
}

async function showLogs(id) {
    const text = await VPSLogs(id);
    outFor = id;
    outLines = String(text || '').replace(/\n$/, '').split('\n').slice(-OUT_MAX);
    if (outLines.length === 1 && !outLines[0]) outLines = [t('(chưa có log)', '(no log yet)')];
}

function onDetailKey(ev) {
    if (ev.key !== 'Enter') return;
    if (ev.target.id === 'vu-name') {
        ev.preventDefault();
        el.body.querySelector('button[data-act="user-add"]')?.click();
    }
}

async function onPickKey() {
    try {
        const p = await PickKeyFile();
        if (p) el.key.value = p;
    } catch (e) {
        setMsg(el.msg, errText(e), true);
    }
}

// poll quietly refreshes the selected VPS (online users, traffic) while the
// tab is visible and nothing else runs; errors are left for the next manual refresh.
let polling = false;
async function poll() {
    const id = selected;
    if (polling || op || !id || !paneVisible() || document.hidden) return;
    if (overviews.get(id)?.state !== 'ok' || !overviews.get(id).data.installed) return;
    polling = true;
    try {
        const data = await VPSOverview(id);
        if (!op) overviews.set(id, {state: 'ok', data: normOverview(data)});
        render();
    } catch { /* keep the last state */ } finally {
        polling = false;
    }
}

/* ---------------- Wiring ---------------- */

export function initVPS(hooks) {
    app = hooks;
    Object.assign(el, {
        list: $('vps-list'),
        msg: $('vps-msg'),
        addBox: $('vps-add-box'),
        form: $('vps-add-box'),
        name: $('vps-name'),
        addr: $('vps-addr'),
        port: $('vps-port'),
        user: $('vps-user'),
        key: $('vps-key'),
        keyRow: $('vps-key-row'),
        keyPick: $('vps-key-pick'),
        pass: $('vps-pass'),
        passLabel: $('vps-pass-label'),
        addBtn: $('vps-add-btn'),
        summary: $('vps-add-summary'),
        cancelEdit: $('vps-edit-cancel'),
        detail: $('vps-detail'),
        title: $('vps-title'),
        refresh: $('vps-refresh'),
        body: $('vps-body'),
        detailMsg: $('vps-detail-msg'),
        out: $('vps-out'),
    });

    el.list.addEventListener('change', (ev) => { if (ev.target.name === 'vps') select(ev.target.value); });
    el.list.addEventListener('click', onListClick);
    el.form.addEventListener('change', (ev) => { if (ev.target.name === 'vps-auth') renderForm(); });
    el.keyPick.addEventListener('click', onPickKey);
    el.addBtn.addEventListener('click', onAddHost);
    el.cancelEdit.addEventListener('click', stopEdit);
    el.pass.addEventListener('keydown', (ev) => { if (ev.key === 'Enter') onAddHost(); });
    el.refresh.addEventListener('click', refresh);
    el.body.addEventListener('click', onDetailClick);
    el.body.addEventListener('keydown', onDetailKey);

    EventsOn('vps-out', (m) => {
        if (!m || m.id !== outFor) return;
        outLines.push(m.line);
        if (outLines.length > OUT_MAX) outLines.splice(0, outLines.length - OUT_MAX);
        if (m.id === selected) {
            el.out.hidden = false;
            el.out.textContent = outLines.join('\n');
            el.out.scrollTop = el.out.scrollHeight;
        }
    });

    loadHosts();
    setInterval(poll, POLL_MS);
}

// renderVPS refreshes the tab when the app's settings change (e.g. "đã có trong app").
export function renderVPS() {
    if (app && paneVisible()) render();
}

// onShow is called when the tab opens: load the selected VPS once.
export function onShowVPS() {
    render();
    ensureLoaded();
}

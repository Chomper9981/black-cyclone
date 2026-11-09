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
} from '../wailsjs/go/main/App';
import {EventsOn, ClipboardSetText} from '../wailsjs/runtime/runtime';

const $ = (id) => document.getElementById(id);
const ARM_MS = 3000;
const OUT_MAX = 400;

const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[c]));
const errText = (e) => {
    const t = e == null ? 'Lỗi không xác định' : typeof e === 'string' ? e : (e.message || String(e));
    return t.charAt(0).toUpperCase() + t.slice(1); // Go errors start lowercase
};
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
        el.list.innerHTML = '<div class="empty">Chưa có VPS. Thêm VPS bên dưới để cài và quản lý server riêng.</div>';
        return;
    }
    el.list.innerHTML = hosts.map((h) => {
        const sel = h.id === selected;
        const delArmed = armed.has('host:' + h.id);
        return `<label class="srv${sel ? ' sel' : ''}">` +
            `<input type="radio" name="vps" value="${esc(h.id)}"${sel ? ' checked' : ''}>` +
            `<span class="srv-main"><span class="srv-name">${esc(h.name)}</span>` +
            `<span class="srv-meta">${esc(h.user)}@${esc(hostPort(h.addr, h.port))} · ${h.auth === 'key' ? 'khoá SSH' : 'mật khẩu'}</span></span>` +
            '<span class="srv-acts">' +
            `<button type="button" class="icon" data-act="host-edit" data-id="${esc(h.id)}" title="Sửa thông tin đăng nhập" aria-label="Sửa">✎</button>` +
            `<button type="button" class="icon del${delArmed ? ' armed' : ''}" data-act="host-del" data-id="${esc(h.id)}" ` +
            `title="${delArmed ? 'Nhấn lần nữa để xoá' : 'Xoá khỏi app (server trên VPS vẫn chạy)'}" aria-label="Xoá">${delArmed ? 'xoá?' : '✕'}</button>` +
            '</span></label>';
    }).join('');
}

function stateBadge(active) {
    if (active === 'active') return '<span class="state on">Đang chạy</span>';
    if (active === 'activating' || active === 'reloading') return '<span class="state warn">Đang khởi động</span>';
    return `<span class="state off">${esc(active === 'failed' ? 'Lỗi' : 'Đã dừng')}</span>`;
}

function renderInstallForm(update) {
    const b = op === 'install';
    return '<div class="vps-install">' +
        (update ? '' : '<p class="hint">Chưa cài server trên VPS này. App sẽ cài sing-box (VLESS + Reality), bật BBR, mở tường lửa và tạo người dùng đầu tiên. Mất khoảng 1–2 phút.</p>' +
            '<label class="field"><span>Tên người dùng đầu tiên</span>' +
            '<input id="vi-name" type="text" spellcheck="false" placeholder="vd. laptop-cua-toi (mặc định: default)"></label>') +
        '<details class="inner"><summary>Tuỳ chọn nâng cao</summary>' +
        '<div class="field-row">' +
        `<label class="field port"><span>Cổng</span><input id="vi-port" type="number" min="1" max="65535" placeholder="${update ? 'giữ nguyên' : '443'}"></label>` +
        `<label class="field grow"><span>Trang giả làm (SNI)</span><input id="vi-sni" type="text" spellcheck="false" placeholder="${update ? 'giữ nguyên' : 'www.microsoft.com'}"></label>` +
        '</div>' +
        `<label class="field"><span>Địa chỉ ghi vào link</span><input id="vi-host" type="text" spellcheck="false" placeholder="${update ? 'giữ nguyên' : 'để trống: IP công khai của VPS'}"></label>` +
        '<p class="hint">Đổi cổng, SNI hoặc địa chỉ thì link cũ của mọi người hết dùng được: gửi lại link mới cho họ.</p>' +
        '</details>' +
        '<div class="row end">' +
        (update ? '<button type="button" class="link" data-act="install-cancel">huỷ</button>' : '') +
        `<button type="button" class="btn sm" data-act="install"${b || op ? ' disabled' : ''}>${b ? 'Đang cài…' : update ? 'Cập nhật server' : 'Cài server'}</button>` +
        '</div></div>';
}

let showUpdate = false;

function renderDetail() {
    const h = hosts.find((x) => x.id === selected);
    el.detail.hidden = !h;
    if (!h) return;
    el.title.textContent = h.name;
    el.refresh.disabled = !!op;
    el.refresh.textContent = op === 'load' ? 'đang tải…' : 'làm mới';

    const ov = overviews.get(h.id);
    let html = '';
    if (!ov || ov.state === 'loading') {
        html = '<div class="empty">Đang kết nối tới VPS…</div>';
    } else if (ov.state === 'error') {
        html = `<div class="ip-err">${esc(ov.error)}</div>`;
        if (ov.error.includes('khoá SSH của VPS đã thay đổi')) {
            const t = armed.has('trust');
            html += '<p class="hint">Chỉ chấp nhận khoá mới nếu chính bạn vừa cài lại hệ điều hành VPS hoặc nhà cung cấp vừa đổi máy.</p>' +
                `<div class="row"><button type="button" class="btn ghost sm danger" data-act="trust"${op ? ' disabled' : ''}>` +
                `${op === 'trust' ? 'Đang kết nối…' : t ? 'Nhấn lần nữa: tôi vừa cài lại VPS' : 'Chấp nhận khoá mới'}</button></div>`;
        }
    } else if (!ov.data.installed) {
        html = renderInstallForm(false);
    } else {
        const o = ov.data;
        const inApp = new Set((app.getServers() || []).map((s) => s.link));
        html += '<div class="ip-grid vps-grid">' +
            `<span>Trạng thái</span>${stateBadge(o.active)}` +
            `<span>sing-box</span><b>${esc(o.version || '?')}</b>` +
            `<span>Địa chỉ</span><code>${esc(hostPort(o.host, o.port))}</code>` +
            `<span>SNI</span><code>${esc(o.sni)}</code>` +
            '</div>';
        if (o.script_outdated && !showUpdate) {
            html += '<div class="vps-note">Bản server trên VPS cũ hơn bản đi kèm app này. ' +
                `<button type="button" class="link" data-act="show-update"${op ? ' disabled' : ''}>cập nhật</button></div>`;
        }

        html += '<div class="vps-sub">Người dùng <span class="count">(' + o.users.length + ')</span></div><div class="sub-list">';
        for (const u of o.users || []) {
            const delArmed = armed.has('user:' + u.name);
            const has = inApp.has(u.link);
            const last = o.users.length <= 1;
            html += '<div class="sub-row">' +
                `<span class="srv-main"><span class="srv-name">${esc(u.name)}</span>` +
                `<span class="srv-meta">${has ? '<span class="in-app">đã có trong app</span>' : 'link riêng của người này'}</span></span>` +
                '<span class="srv-acts">' +
                `<button type="button" class="icon${qr && qr.id === h.id && qr.name === u.name ? ' on' : ''}" data-act="qr" data-name="${esc(u.name)}" title="Hiện mã QR để điện thoại quét">QR</button>` +
                `<button type="button" class="icon" data-act="copy" data-name="${esc(u.name)}" title="Copy link để gửi cho người này">copy</button>` +
                (has ? '' : `<button type="button" class="icon" data-act="import" data-name="${esc(u.name)}" title="Thêm link này vào danh sách server của app trên máy này">+ app</button>`) +
                (last ? '' : `<button type="button" class="icon del${delArmed ? ' armed' : ''}" data-act="user-del" data-name="${esc(u.name)}" ` +
                    `title="${delArmed ? 'Nhấn lần nữa để xoá' : 'Xoá người dùng: link của họ hết dùng được ngay'}" aria-label="Xoá"${op ? ' disabled' : ''}>${delArmed ? 'xoá?' : '✕'}</button>`) +
                '</span></div>';
            if (qr && qr.id === h.id && qr.name === u.name) {
                html += '<div class="qr-box">' +
                    `<img src="${esc(qr.src)}" alt="Mã QR link của ${esc(u.name)}" width="220" height="220">` +
                    `<p class="hint">Trên điện thoại của <b>${esc(u.name)}</b>: quét bằng camera hoặc Google Lens, copy link rồi dán vào Black Cyclone → Server riêng. ` +
                    'Mã QR chứa link đầy đủ: ai chụp được nó cũng dùng được server.</p></div>';
            }
        }
        html += '</div>';
        html += '<div class="row">' +
            '<input id="vu-name" type="text" spellcheck="false" placeholder="Tên người dùng mới, vd. dien-thoai-me">' +
            `<button type="button" class="btn sm" data-act="user-add"${op ? ' disabled' : ''}>${op === 'user-add' ? 'Đang thêm…' : 'Thêm'}</button></div>` +
            '<p class="hint">Mỗi người / thiết bị một tên. Muốn cắt quyền ai thì chỉ cần xoá đúng tên đó.</p>';

        if (showUpdate) html += renderInstallForm(true);

        const unArmed = armed.has('uninstall');
        html += '<div class="vps-actions">' +
            `<button type="button" class="btn ghost sm" data-act="restart"${op ? ' disabled' : ''}>${op === 'restart' ? 'Đang khởi động lại…' : 'Khởi động lại'}</button>` +
            `<button type="button" class="btn ghost sm" data-act="logs"${op ? ' disabled' : ''}>${op === 'logs' ? 'Đang tải…' : 'Xem log'}</button>` +
            (showUpdate ? '' : `<button type="button" class="btn ghost sm" data-act="show-update"${op ? ' disabled' : ''}>Cập nhật / đổi cấu hình</button>`) +
            `<button type="button" class="btn ghost sm danger" data-act="uninstall"${op ? ' disabled' : ''}>` +
            `${op === 'uninstall' ? 'Đang gỡ…' : unArmed ? 'Nhấn lần nữa: gỡ hết, mọi link mất hiệu lực' : 'Gỡ server'}</button>` +
            '</div>';
    }
    html += `<div class="vps-fp">Khoá SSH: <code>${esc(h.fingerprint || '?')}</code></div>`;

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
    el.passLabel.textContent = key ? 'Mật khẩu (của khoá hoặc sudo, nếu có)' : 'Mật khẩu';
    el.addBtn.disabled = adding;
    el.addBtn.textContent = adding ? 'Đang kiểm tra…' : editing ? 'Lưu thay đổi' : 'Kiểm tra & lưu';
    const h = editing && hosts.find((x) => x.id === editing);
    el.summary.textContent = h ? `Sửa VPS: ${h.name}` : 'Thêm VPS';
    el.cancelEdit.hidden = !editing;
    el.pass.placeholder = editing ? 'để trống: giữ mật khẩu đã lưu' : '';
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
    if (!input.addr) { setMsg(el.msg, 'Nhập IP hoặc tên miền của VPS.', true); return; }
    if (auth === 'key' && !input.key_file) { setMsg(el.msg, 'Chọn file khoá SSH.', true); return; }
    const old = editing && hosts.find((x) => x.id === editing);
    // Editing: an empty password keeps the saved one (unless switching to password login).
    if (auth !== 'key' && !input.password && !(old && old.auth === 'password')) {
        setMsg(el.msg, 'Nhập mật khẩu SSH.', true);
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
        setMsg(el.msg, id ? `Đã lưu ${v.name}.` : `Đã thêm ${v.name}. Khoá SSH của VPS: ${v.fingerprint}`);
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
        setMsg(el.msg, 'Đã xoá VPS khỏi app. Server trên VPS vẫn chạy, link của mọi người vẫn dùng được.');
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
            setMsg(el.detailMsg, 'Không tạo được mã QR: ' + errText(e), true);
        }
        render();
        return;
    case 'copy':
        if (!user) return;
        try {
            await ClipboardSetText(user.link);
            setMsg(el.detailMsg, `Đã copy link của ${name}. Gửi riêng cho người đó, đừng đăng công khai.`);
        } catch (e) {
            setMsg(el.detailMsg, 'Không copy được: ' + errText(e), true);
        }
        return;
    case 'import':
        if (!user) return;
        try {
            await app.importLinks(user.link);
            setMsg(el.detailMsg, `Đã thêm link của ${name} vào tab Kết nối → Server riêng.`);
        } catch (e) {
            setMsg(el.detailMsg, errText(e), true);
        }
        render();
        return;
    case 'user-add': {
        const inp = $('vu-name');
        const n = (inp?.value || '').trim();
        if (!/^[A-Za-z0-9._-]{1,32}$/.test(n)) {
            setMsg(el.detailMsg, 'Tên chỉ gồm chữ không dấu, số, ".", "_", "-" (tối đa 32 ký tự).', true);
            return;
        }
        const r = await run('user-add', (id) => VPSAddUser(id, n), `Đã thêm ${n}. Bấm "copy" để lấy link gửi cho họ.`);
        if (r && inp) { const fresh = $('vu-name'); if (fresh) fresh.value = ''; }
        return;
    }
    case 'user-del': {
        const key = 'user:' + name;
        if (!armed.has(key)) { arm(key); return; }
        disarm(key);
        if (qr && qr.name === name) qr = null;
        run('user-del', (id) => VPSRemoveUser(id, name), `Đã xoá ${name}: link của người này hết dùng được.`);
        return;
    }
    case 'trust': {
        if (!armed.has('trust')) { arm('trust'); return; }
        disarm('trust');
        run('trust', async (id) => {
            await TrustNewVPSKey(id);
            await loadHosts(); // shows the new fingerprint
            return VPSOverview(id);
        }, 'Đã chấp nhận khoá SSH mới của VPS.');
        return;
    }
    case 'restart':
        run('restart', VPSRestart, 'Đã khởi động lại server.');
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
            setMsg(el.detailMsg, 'Tên chỉ gồm chữ không dấu, số, ".", "_", "-" (tối đa 32 ký tự).', true);
            return;
        }
        outFor = selected;
        outLines = [];
        const r = await run('install', (id) => VPSInstall(id, opts), update ? 'Đã cập nhật server.' : 'Đã cài server. Thêm link vào app bằng nút "+ app".');
        if (r) showUpdate = false;
        render();
        return;
    }
    case 'uninstall':
        if (!armed.has('uninstall')) { arm('uninstall'); return; }
        disarm('uninstall');
        showUpdate = false;
        run('uninstall', VPSUninstall, 'Đã gỡ server khỏi VPS.');
        return;
    }
}

async function showLogs(id) {
    const text = await VPSLogs(id);
    outFor = id;
    outLines = String(text || '').replace(/\n$/, '').split('\n').slice(-OUT_MAX);
    if (outLines.length === 1 && !outLines[0]) outLines = ['(chưa có log)'];
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

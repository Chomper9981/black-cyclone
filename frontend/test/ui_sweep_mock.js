// Mock backend for an error sweep: every mode / account type / language / tab.
const q = new URLSearchParams(location.search);
const errs = [];
addEventListener('error', (e) => errs.push('error: ' + (e.message || e)));
addEventListener('unhandledrejection', (e) => errs.push('rejection: ' + (e.reason && e.reason.message || e.reason)));
localStorage.setItem('rcv-tab', q.get('tab') || 'conn');
const mode = q.get('mode') || 'warp';
const acct = q.get('acct');   // none | free | unlimited | limited | other
const warp = acct === 'none' ? null : {private_key: 'k', peer_public_key: 'p', endpoint: q.get('ep') ? '162.159.192.1' : '', endpoint_port: 2408,
  account_type: acct === 'other' ? 'team' : (acct === 'free0' ? '' : acct), addresses: []};
const running = q.get('run') === '1';
const A = {
  GetLogs: async () => ['line one'],
  GetStatus: async () => ({running, state: running ? (q.get('state') || 'on') : 'off', mode, since: new Date(Date.now() - 65000).toISOString(), admin: q.get('admin') !== '0',
    bins_ok: q.get('bins') !== '0', missing: q.get('bins') === '0' ? ['sing-box.exe'] : [], os: q.get('os') || 'windows', server: 'Singapore', warnings: running ? ['skipped server "x": bad'] : [],
    error: q.get('err') ? 'boom' : '', blocking: q.get('state') === 'blocked', pending: q.get('pending') === '1', lang: q.get('lang') || 'vi'}),
  GetSettings: async () => ({mode, warp, servers: [{id: 's1', name: 'Singapore', link: 'vless://a@1.2.3.4:443#x'}, {id: 's2', name: '', link: 'hysteria2://e@192.0.2.44:443#hk', sub_id: 'u1'}],
    subscriptions: [{id: 'u1', name: 'My sub', url: 'https://x', updated: new Date(Date.now() - 7200000).toISOString()}], selected_server: 's1', routing: {direct_vn: true}, start_with_windows: true}),
  PingServers: async () => ({s1: 38, s2: -2}),
  ScanWarpEndpoints: async () => [{ip: '162.159.192.1', port: 2408, ms: 40}, {ip: '162.159.193.5', port: 500, ms: 420}],
  CheckConnection: async () => ({ip: {ip: '1.2.3.4', country: 'SG', colo: 'SIN', warp: 'off'}, sites: [{name: 'Google', ok: true, ms: 80}, {name: 'X', ok: false, error: 'timed out'}]}),
  ListVPS: async () => [{id: 'v1', name: 'SG', addr: '203.0.113.10', port: 22, user: 'root', auth: 'key', fingerprint: 'SHA256:x'}],
  VPSOverview: async () => { if (q.get('vps') === 'hk') throw "the VPS's SSH key has changed (was: a, now: b)."; if (q.get('vps') === 'none') return {installed: false};
    return {installed: true, active: q.get('vps') === 'failed' ? 'failed' : 'active', host: '203.0.113.10', port: 443, sni: 'www.microsoft.com', version: '1.14.3', script_outdated: true, stats: true, online: true,
      users: [{name: 'a', link: 'vless://a@1.2.3.4:443#x', up: 1, down: 2e9, conns: 2, ips: ['1.1.1.1', '2.2.2.2', '3.3.3.3']}, {name: 'b', link: 'vless://b@1.2.3.4:443#y', up: 0, down: 0, conns: 0, ips: []}]}; },
  VPSLinkQR: async () => 'data:image/gif;base64,R0lGODlhAQABAAAAACw=',
};
window.go = {main: {App: new Proxy(A, {get: (t, k) => t[k] || (async () => null)})}};
const subs = {};
window.runtime = {EventsOnMultiple: (n, cb) => { subs[n] = cb; return () => {}; }, ClipboardSetText: async () => true};
addEventListener('load', () => setTimeout(async () => {
  try {
    if (running) subs.stats && subs.stats({up: 1, down: 2, up_total: 3, down_total: 4});
    // Click through things that render more text.
    for (const id of ['srv-ping', 'check-btn', 'warp-scan']) { const b = document.getElementById(id); if (b && !b.disabled) b.click(); }
    for (const b of document.querySelectorAll('button.help')) b.click();
    await new Promise((r) => setTimeout(r, 300));
    for (const sel of ['button[data-act="qr"]', 'button[data-act="show-update"]', 'button[data-act="reset"]', 'button[data-act="uninstall"]', 'button[data-act="host-edit"]', 'button[data-act="trust"]', '#warp-reregister', 'button[data-act="del"]', 'button[data-act="rename"]']) {
      const b = document.querySelector(sel); if (b && !b.disabled) b.click();
    }
    await new Promise((r) => setTimeout(r, 300));
  } catch (e) { errs.push('driver: ' + e.message); }
  const o = document.createElement('script'); o.type = 'application/json'; o.id = 'errs';
  o.textContent = JSON.stringify({errs, pill: document.getElementById('pill').textContent, toggle: document.getElementById('toggle-label').textContent, disabled: document.getElementById('toggle').disabled});
  document.body.appendChild(o);
}, 700));

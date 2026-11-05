package singbox

// platformTun adjusts the TUN inbound for Android, where the app's VpnService
// creates the interface (addresses, routes, per-app rules) and hands its fd to
// sing-box (mobile/core/rcvcore):
//   - stack "gvisor": the "system" stack answers TCP from kernel sockets owned
//     by this app, which is excluded from its own VPN, so replies would skip the
//     TUN. gVisor keeps everything in userspace.
//   - auto_route stays on so sing-box treats the interface as carrying the
//     default route; the routes themselves are set by the VpnService.
func platformTun(tun map[string]any) {
	tun["stack"] = "gvisor"
	delete(tun, "strict_route")
	delete(tun, "interface_name")
}

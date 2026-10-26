package singbox

// platformTun adjusts the TUN inbound for Linux desktops:
//   - stack "gvisor": the "system" half of "mixed" hands TCP back to the kernel
//     through the TUN, where a host firewall (UFW's default INPUT deny) drops it,
//     leaving only QUIC working.
//   - auto_redirect: nftables sends TCP and DNS straight to sing-box. This also
//     catches systemd-resolved, whose queries are bound to the physical
//     interface and would otherwise skip the TUN and our DoH resolver.
func platformTun(tun map[string]any) {
	tun["stack"] = "gvisor"
	tun["auto_redirect"] = true
}

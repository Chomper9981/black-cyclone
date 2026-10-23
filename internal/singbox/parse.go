package singbox

import "redcyclone-vpn/internal/share"

func parseLink(link, tag string) (map[string]any, error) {
	return share.ParseOutbound(link, tag)
}

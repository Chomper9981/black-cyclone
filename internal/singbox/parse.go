package singbox

import "black-cyclone/internal/share"

func parseLink(link, tag string) (map[string]any, error) {
	return share.ParseOutbound(link, tag)
}

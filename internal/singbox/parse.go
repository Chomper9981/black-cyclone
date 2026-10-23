package singbox

import "redcyclone-vpn/internal/vless"

func parseLink(link, tag string) (map[string]any, error) {
	ob, err := vless.ParseOutbound(link)
	if err != nil {
		return nil, err
	}
	ob["tag"] = tag
	return ob, nil
}

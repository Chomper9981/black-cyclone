package warp

// Account is a registered WARP device. The JSON layout matches settings.Warp of the
// desktop app, so an account can be copied between the two.
type Account struct {
	PrivateKey    string   `json:"private_key"`
	PeerPublicKey string   `json:"peer_public_key"`
	Endpoint      string   `json:"endpoint"`
	EndpointPort  int      `json:"endpoint_port"`
	Addresses     []string `json:"addresses"`
	Reserved      []int    `json:"reserved"`
	// Device ID and API token from registration, needed to apply a WARP+ license.
	DeviceID string `json:"device_id,omitempty"`
	Token    string `json:"token,omitempty"`
	License  string `json:"license,omitempty"`
	// AccountType as reported by the API: "free", "unlimited" (WARP+), ...
	AccountType string `json:"account_type,omitempty"`
}

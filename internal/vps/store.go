package vps

import "black-cyclone/internal/settings"

const storeFile = "vps.json"

type store struct {
	Hosts []Host `json:"hosts"`
}

// Load returns the saved hosts (with secrets).
func Load() ([]Host, error) {
	var s store
	err := settings.LoadSecret(storeFile, &s)
	return s.Hosts, err
}

// Save encrypts and stores hosts.
func Save(hosts []Host) error {
	return settings.SaveSecret(storeFile, store{Hosts: hosts})
}

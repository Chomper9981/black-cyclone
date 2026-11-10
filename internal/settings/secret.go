package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"black-cyclone/internal/i18n"
)

// LoadSecret reads another encrypted JSON file from the settings folder into v
// (e.g. vps.json with SSH credentials, kept apart from settings.json so it never
// travels to the UI). A missing file leaves v untouched.
func LoadSecret(name string, v any) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(data), encMagic) {
		return errors.New(name + i18n.T(": file không đúng định dạng", ": not a valid file"))
	}
	if data, err = unprotect(data[len(encMagic):]); err != nil {
		return errors.New(i18n.T("không giải mã được ", "cannot decrypt ") + name + ": " + err.Error())
	}
	return json.Unmarshal(data, v)
}

// SaveSecret encrypts v as JSON into name in the settings folder.
func SaveSecret(name string, v any) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	enc, err := protect(data)
	if err != nil {
		return err
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p+".tmp", append([]byte(encMagic), enc...), 0o600); err != nil {
		return err
	}
	return os.Rename(p+".tmp", p)
}

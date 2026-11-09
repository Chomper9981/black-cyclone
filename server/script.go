// Package server ships bcserver.sh inside the apps (desktop and Android), so
// installing or updating a VPS from the app always uses the matching script.
package server

import _ "embed"

// Script is bcserver.sh as checked out (line endings may be CRLF on Windows;
// vps.Script normalises them).
//
//go:embed bcserver.sh
var Script []byte

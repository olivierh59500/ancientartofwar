// Package assets embeds the supplied game data for desktop and Android.
package assets

import _ "embed"


// Disk bundles the original game data for desktop and Android.
//
//go:embed original/disk1.img
var Disk []byte

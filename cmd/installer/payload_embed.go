//go:build installer_bundle

package main

import _ "embed"

//go:embed payload/ATLOB_Sync.exe.gz
var appPayloadGz []byte

//go:embed payload/Desinstalar_ATLOB_Sync.exe.gz
var uninstallerPayloadGz []byte

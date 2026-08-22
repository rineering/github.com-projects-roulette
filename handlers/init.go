package handlers

import (
	"embed"
)

var staticFS *embed.FS

// InitStatic сохраняет вшитую файловую систему внутри пакета handlers
func InitStatic(fs *embed.FS) {
	staticFS = fs
}
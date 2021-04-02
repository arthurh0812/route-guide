package data

import (
	"path/filepath"
	"runtime"
)

// basepath is the path of this directory
var basepath string

func init() {
	_, currentFile, _, _ := runtime.Caller(0)
	basepath = filepath.Dir(currentFile)
}

// Path joins the provided relative (or absolute) filepath with the x509 files inside inside x509/
func Path(rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(basepath, rel)
}
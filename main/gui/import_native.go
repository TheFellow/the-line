//go:build !js

package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (g *game) chooseCSV() {
	path := g.opts.file
	if !strings.HasSuffix(strings.ToLower(path), ".csv") {
		path += ".csv"
	}
	file, err := os.Open(path)
	var data []byte
	if err == nil {
		data, err = io.ReadAll(io.LimitReader(file, (2<<20)+1))
		file.Close()
	}
	g.importRequests <- csvImport{data: string(data), name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), err: err}
}

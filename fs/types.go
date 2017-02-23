// Copyright 2016-2017 Brandon Buck

package fs

import (
	"os"
	"path/filepath"

	"github.com/bbuck/dragon-mud/assets"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/text/tmpl"
)

// ItemType represents the kind of item, such as a File or Directory
type ItemType uint8

// Item types
const (
	ItemTypeFile ItemType = 1 + iota
	ItemTypeDir
)

// Item is an interface that both File and Dir implement.
type Item interface {
	Type() ItemType
	Create(logger.Log, string, interface{})
}

// File wraps a string and represents a file with that name in the given
// location on the file system.
type File struct{}

// Type for File returns ItemTypeFile.
func (File) Type() ItemType {
	return ItemTypeFile
}

// Create will create the file with asset data for the given file name.
func (File) Create(log logger.Log, fpath string, templateData interface{}) {
	flog := log.WithField("filename", fpath)

	assetName := filepath.Base(fpath)
	fdata := assets.MustAsset(assetName)
	fdata = []byte(tmpl.MustRenderOnce(string(fdata), templateData))

	flog.Info("Creating file     ")
	file, err := os.Create(fpath)
	if err != nil {
		flog.WithError(err).Fatal("Failed to create file in the current directory.")

		return
	}
	defer file.Close()

	n, werr := file.Write(fdata)
	if werr != nil {
		flog.WithError(werr).Fatal("Failed to write a default file.")
	} else if n != len(fdata) {
		flog.WithField("percentage", (float64(n) / float64(len(fdata)) * 100.0)).Fatal("Failed to write the entire file.")
	}
}

// Dir is a map of Items. This allows you to define in a simplified way
// complex (or simple) file structure.
type Dir map[string]Item

// Type for Dir returns ItemTypeDir.
func (Dir) Type() ItemType {
	return ItemTypeDir
}

// Create defines how how a directory is to be created, which is simply
// creating the directory and then calling CreateFromStructure with the
// directory.
func (d Dir) Create(log logger.Log, fpath string, td interface{}) {
	dlog := log.WithField("dirname", fpath)

	dlog.Info("Creating directory")
	err := os.Mkdir(fpath, os.ModePerm)
	if err != nil {
		dlog.WithError(err).Fatal("Failed to create directory.")
	}

	CreateFromStructure(CreateStructureParams{
		Log:          log,
		BaseName:     fpath,
		Structure:    d,
		TemplateData: td,
	})
}

package install

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
)

// Hash returns a hash of every file in fsys: their paths and contents,
// in path order. It's what dragon.lock records, so any change to an
// installed plugin's files changes it.
func Hash(fsys fs.FS) (string, error) {
	sum := sha256.New()
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		fmt.Fprintf(sum, "%s\x00%d\x00", name, len(data))
		sum.Write(data)
		return nil
	})
	if err != nil {
		return "", err
	}

	return "sha256:" + hex.EncodeToString(sum.Sum(nil)), nil
}

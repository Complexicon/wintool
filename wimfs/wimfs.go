package wimfs

import (
	"fmt"
	"io"
	"io/fs"
	"reflect"
	"strings"
	"time"

	"github.com/Microsoft/go-winio/wim"
)

type WimEntry struct {
	file       *wim.File
	reader     io.ReadCloser
	subentries []fs.DirEntry
}

var (
	_ fs.DirEntry    = (*WimEntry)(nil)
	_ fs.FileInfo    = (*WimEntry)(nil)
	_ fs.ReadDirFile = (*WimEntry)(nil)
)

func (e *WimEntry) Offset() int64 {
	return reflect.ValueOf(*e.file).FieldByName("offset").FieldByName("Offset").Int()
}

func (entry *WimEntry) buildFileTree() error {

	if entry.file.IsDir() {
		files, err := entry.file.Readdir()
		if err != nil {
			return fmt.Errorf("%s: %w", entry.file.Name, err)
		}

		entry.subentries = make([]fs.DirEntry, 0)

		for _, v := range files {

			var subfile = &WimEntry{file: v}
			err := subfile.buildFileTree()

			if err != nil {
				return err
			}

			entry.subentries = append(entry.subentries, subfile)

		}

	}

	return nil
}

func CreateWimFS(image *wim.Image) (fs.FS, error) {

	var rootDescriptor, err = image.Open()

	if err != nil {
		return nil, err
	}

	var rootFile = &WimEntry{file: rootDescriptor}

	/*
		must precalculate file tree because microsoft's
		implementation of wim files for go
		allows only forward reading in images
		and no lazy access
	*/
	err = rootFile.buildFileTree()

	if err != nil {
		return nil, err
	}

	return rootFile, nil
}

func (u *WimEntry) initReader() {
	rdr, err := u.file.Open()

	if err != nil {
		panic(err)
	}

	u.reader = rdr

}

// WriteTo implements io.WriterTo.
func (u *WimEntry) WriteTo(w io.Writer) (int64, error) {
	var buffer = make([]byte, 1024*1024*16)
	var read, err = u.Read(buffer)
	var total = 0

	for err == nil {
		written, wrErr := w.Write(buffer[:read])

		if wrErr != nil {
			return int64(written), err
		}

		if written != read {
			return int64(written), fmt.Errorf("read %d bytes but only written %d", read, written)
		}

		total += written

		read, err = u.Read(buffer)

	}

	return int64(total), nil
}

// Info implements fs.DirEntry.
func (u *WimEntry) Info() (fs.FileInfo, error) {
	return u, nil
}

// Type implements fs.DirEntry.
func (u *WimEntry) Type() fs.FileMode {
	return u.Mode()
}

func (u *WimEntry) ReadDir(n int) ([]fs.DirEntry, error) {
	if u.subentries == nil {
		return nil, fmt.Errorf("not a directory")
	}
	return u.subentries, nil
}

// IsDir implements fs.FileInfo.
func (w *WimEntry) IsDir() bool {
	return w.file.IsDir()
}

// ModTime implements fs.FileInfo.
func (w *WimEntry) ModTime() time.Time {
	return w.file.LastWriteTime.Time()
}

// Mode implements fs.FileInfo.
func (w *WimEntry) Mode() fs.FileMode {
	return 0444
}

// Name implements fs.FileInfo.
func (w *WimEntry) Name() string {
	return w.file.Name
}

// Size implements fs.FileInfo.
func (w *WimEntry) Size() int64 {
	return w.file.Size
}

// Sys implements fs.FileInfo.
func (w *WimEntry) Sys() any {
	return nil
}

// Close implements fs.File.
func (w *WimEntry) Close() error {
	if w.reader != nil {
		return w.reader.Close()
	} else {
		return nil
	}
}

// Read implements fs.File.
func (w *WimEntry) Read(p []byte) (int, error) {
	if w.reader == nil {
		w.initReader()
	}

	return w.reader.Read(p)
}

// Stat implements fs.File.
func (w *WimEntry) Stat() (fs.FileInfo, error) {
	return w, nil
}

// Open implements fs.FS.
func (w *WimEntry) Open(name string) (fs.File, error) {
	var file = w

	for _, v := range strings.Split(name, "/") {

		if v == "." {
			continue
		}

		if file.subentries == nil {
			return nil, fmt.Errorf("%s not a folder", file.file.Name)
		}

		var newFile *WimEntry

		for _, e := range file.subentries {
			if e.Name() == v {
				newFile = e.(*WimEntry)
				break
			}
		}

		if newFile == nil {
			return nil, fmt.Errorf("folder %s not found", v)
		}

		file = newFile

	}

	return file, nil
}

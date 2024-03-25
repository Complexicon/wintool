package udf

import (
	"fmt"
	"io"
	"io/fs"
	"time"
)

var (
	_ fs.DirEntry    = (*UdfFile)(nil)
	_ fs.FileInfo    = (*UdfFile)(nil)
	_ fs.ReadDirFile = (*UdfFile)(nil)
	_ io.ReaderAt    = (*UdfFile)(nil)
	_ io.WriterTo    = (*UdfFile)(nil)
)

type UdfFile struct {
	Udf               *udfDescriptor
	Fid               *FileIdentifierDescriptor
	fe                *FileEntry
	fileEntryPosition uint64
	reader            *io.SectionReader
}

// WriteTo implements io.WriterTo.
func (u *UdfFile) WriteTo(w io.Writer) (int64, error) {
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

func (u *UdfFile) initReader() {
	u.reader = io.NewSectionReader(u.Udf.reader, u.GetFileOffset(), u.Size())
}

// ReadAt implements io.ReaderAt.
func (u *UdfFile) ReadAt(p []byte, off int64) (n int, err error) {

	if u.reader == nil {
		u.initReader()
	}

	return u.reader.ReadAt(p, off)
}

// Close implements fs.ReadDirFile.
func (u *UdfFile) Close() error {
	if u.reader != nil {
		u.reader = nil
	}
	return nil
}

// Read implements fs.ReadDirFile.
func (u *UdfFile) Read(p []byte) (int, error) {
	if u.reader == nil {
		u.initReader()
	}

	return u.reader.Read(p)
}

// ReadDir implements fs.ReadDirFile.
func (u *UdfFile) ReadDir(n int) ([]fs.DirEntry, error) {
	ps := u.Udf.PartitionStart()

	adPos := u.FileEntry().AllocationDescriptors[0]
	fdLen := uint64(adPos.Length)

	fdBuf := u.Udf.ReadSectors(ps+uint64(adPos.Location), (fdLen+SECTOR_SIZE-1)/SECTOR_SIZE)
	fdOff := uint64(0)

	result := make([]fs.DirEntry, 0)

	for uint32(fdOff) < adPos.Length {
		fid := NewFileIdentifierDescriptor(fdBuf[fdOff:])
		if fid.FileIdentifier != "" {
			result = append(result, &UdfFile{
				Udf: u.Udf,
				Fid: fid,
			})
		}
		fdOff += fid.Len()
	}

	return result, nil
}

// Stat implements fs.ReadDirFile.
func (u *UdfFile) Stat() (fs.FileInfo, error) {
	return u, nil
}

// ModTime implements fs.FileInfo.
func (u *UdfFile) ModTime() time.Time {
	return u.FileEntry().ModificationTime
}

// Mode implements fs.FileInfo.
func (u *UdfFile) Mode() fs.FileMode {
	return 0444
}

// Size implements fs.FileInfo.
func (u *UdfFile) Size() int64 {
	return int64(u.FileEntry().InformationLength)
}

// Sys implements fs.FileInfo.
func (u *UdfFile) Sys() any {
	return nil
}

// Info implements fs.DirEntry.
func (u *UdfFile) Info() (fs.FileInfo, error) {
	return u, nil
}

// IsDir implements fs.DirEntry.
func (u *UdfFile) IsDir() bool {
	return u.FileEntry().ICBTag.FileType == 4
}

// Name implements fs.DirEntry.
func (u *UdfFile) Name() string {
	return u.Fid.FileIdentifier
}

// Type implements fs.DirEntry.
func (u *UdfFile) Type() fs.FileMode {
	return u.Mode()
}

func (f *UdfFile) GetFileEntryPosition() int64 {
	return int64(f.fileEntryPosition)
}

func (f *UdfFile) GetFileOffset() int64 {
	return SECTOR_SIZE * (int64(f.FileEntry().AllocationDescriptors[0].Location) + int64(f.Udf.PartitionStart()))
}

func (f *UdfFile) FileEntry() *FileEntry {
	if f.fe == nil {
		f.fileEntryPosition = f.Fid.ICB.Location
		f.fe = NewFileEntry(f.Udf.ReadSector(f.Udf.PartitionStart() + f.fileEntryPosition))
	}
	return f.fe
}

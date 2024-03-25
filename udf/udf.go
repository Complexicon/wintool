package udf

import (
	"fmt"
	"io"
	"io/fs"
	"strings"
)

const SECTOR_SIZE = 2048

type udfDescriptor struct {
	reader io.ReaderAt
	pvd    *PrimaryVolumeDescriptor
	pd     *PartitionDescriptor
	lvd    *LogicalVolumeDescriptor
	fsd    *FileSetDescriptor
	root   *FileEntry
}

func (udf *udfDescriptor) Open(name string) (fs.File, error) {
	var file = &UdfFile{fe: udf.root, Udf: udf}

	for _, v := range strings.Split(name, "/") {

		if v == "." {
			continue
		}

		var fi, err = file.Stat()

		if err != nil {
			return nil, err
		}

		if !fi.IsDir() {
			return nil, fmt.Errorf("folder %s is not a folder", v)
		}

		entries, err := file.ReadDir(0)

		if err != nil {
			return nil, err
		}

		var newFile *UdfFile

		for _, e := range entries {
			if e.Name() == v {
				newFile = e.(*UdfFile)
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

func (udf *udfDescriptor) PartitionStart() uint64 {
	if udf.pd == nil {
		panic(udf)
	} else {
		return uint64(udf.pd.PartitionStartingLocation)
	}
}

func (udf *udfDescriptor) GetReader() io.ReaderAt {
	return udf.reader
}

func (udf *udfDescriptor) ReadSectors(sectorNumber uint64, sectorsCount uint64) []byte {
	buf := make([]byte, SECTOR_SIZE*sectorsCount)
	readBytes, err := udf.reader.ReadAt(buf[:], int64(SECTOR_SIZE*sectorNumber))
	if err != nil {
		panic(err)
	}
	if readBytes != int(SECTOR_SIZE*sectorsCount) {
		panic(readBytes)
	}
	return buf[:]
}

func (udf *udfDescriptor) ReadSector(sectorNumber uint64) []byte {
	return udf.ReadSectors(sectorNumber, 1)
}

func NewFromReader(r io.ReaderAt) (fs.FS, error) {
	udf := &udfDescriptor{reader: r}

	anchorDesc := NewAnchorVolumeDescriptorPointer(udf.ReadSector(256))
	if anchorDesc.Descriptor.TagIdentifier != DESCRIPTOR_ANCHOR_VOLUME_POINTER {
		return nil, fmt.Errorf("first descriptor was not an anchor volume descriptor")
	}

	for sector := uint64(anchorDesc.MainVolumeDescriptorSeq.Location); ; sector++ {
		desc := NewDescriptor(udf.ReadSector(sector))
		if desc.TagIdentifier == DESCRIPTOR_TERMINATING {
			break
		}
		switch desc.TagIdentifier {
		case DESCRIPTOR_PRIMARY_VOLUME:
			udf.pvd = desc.PrimaryVolumeDescriptor()
		case DESCRIPTOR_PARTITION:
			udf.pd = desc.PartitionDescriptor()
		case DESCRIPTOR_LOGICAL_VOLUME:
			udf.lvd = desc.LogicalVolumeDescriptor()
		}
	}

	partitionStart := udf.PartitionStart()

	udf.fsd = NewFileSetDescriptor(udf.ReadSector(partitionStart + udf.lvd.LogicalVolumeContentsUse.Location))
	udf.root = NewFileEntry(udf.ReadSector(partitionStart + udf.fsd.RootDirectoryICB.Location))

	return udf, nil
}

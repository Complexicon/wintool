package main

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"sort"

	"git.cmplx.dev/wintool/httpreader"
	"git.cmplx.dev/wintool/msiso"
	"git.cmplx.dev/wintool/udf"
	"git.cmplx.dev/wintool/wimfs"

	"github.com/Microsoft/go-winio/wim"
)

func main() {

	/*

		TODOS:
			cmdline flags
			creating ntfs with ntfs-3g
			saving extended attributes
			making disk bootable
			uefi/bios support

	*/

	var dl = msiso.GetDownload(msiso.Win10)

	_, err := dl.GetLanguages()

	if err != nil {
		log.Panic(err)
	}

	dlurl, err := dl.GetISOLink("German")

	if err != nil {
		log.Panic(err)
	}

	log.Println(dlurl)

	isoReader, err := httpreader.CreateHTTPReader(dlurl, 0, -1)

	if err != nil {
		log.Panic(err)
	}

	iso, err := udf.NewFromReader(isoReader)

	if err != nil {
		log.Panic(err)
	}

	installWim, err := iso.Open("sources/install.wim")

	if err != nil {
		log.Panic(err)
	}

	wimFile, err := wim.NewReader(installWim.(io.ReaderAt))

	if err != nil {
		log.Panic(err)
	}

	log.Println("building wim file tree... this might take a while")
	wimFS, err := wimfs.CreateWimFS(wimFile.Image[0])

	if err != nil {
		log.Panic(err)
	}

	os.Mkdir("windows_target", 0777)

	type ToUncompress struct {
		file *wimfs.WimEntry
		path string
	}

	var filesToUncompress = make([]ToUncompress, 0)

	log.Println("creating directory structure...")
	fs.WalkDir(wimFS, ".", func(path string, d fs.DirEntry, err error) error {

		if d.IsDir() {
			os.Mkdir("windows_target/"+path, 0777)
		} else {
			filesToUncompress = append(filesToUncompress, ToUncompress{file: d.(*wimfs.WimEntry), path: path})
		}

		return nil
	})

	log.Printf("having to unpack %d files", len(filesToUncompress))

	sort.Slice(filesToUncompress, func(i, j int) bool {
		return filesToUncompress[i].file.Offset() < filesToUncompress[j].file.Offset()
	})

	log.Println("begin unpack...")

	var totalFiles = float64(len(filesToUncompress))
	var lastPercentage = 0

	for i, v := range filesToUncompress {
		var out, _ = os.Create("windows_target/" + v.path)
		io.Copy(out, v.file)
		v.file.Close()
		out.Close()
		var newPercentage = int(float64(i+1) / totalFiles * 10000)
		if lastPercentage != newPercentage {
			fmt.Printf("\033[2K\r%.2f done", float64(newPercentage)/100)
			lastPercentage = newPercentage
		}
	}

}

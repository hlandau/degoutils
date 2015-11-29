package binarc

import "sync"
import "os"
import "gopkg.in/hlandau/svcutils.v1/exepath"
import "github.com/hlandau/degoutils/vfs"
import "github.com/hlandau/degoutils/vfs/zipfs"
import "github.com/hlandau/xlog"

var log, Log = xlog.New("binarc")

var setupOnce sync.Once
var inlineArchive vfs.Filesystem

func Setup(path string) error {
	setupOnce.Do(func() {
		f, err := openSelfFile()
		if err != nil {
			log.Errore(err, "cannot open self")
			return
		}

		finfo, err := f.Stat()
		if err != nil {
			f.Close()
			log.Errore(err, "cannot stat self")
			return
		}

		arc, err := zipfs.New(f, finfo.Size())
		if err != nil {
			f.Close()
			return
		}

		inlineArchive = arc

		mount := &vfs.Mounter{}
		err = mount.Mount(path, inlineArchive)
		if err != nil {
			log.Errore(err, "mount")
			return
		}

		vfs.Default = &vfs.Overlay{
			Overlay:  mount,
			Underlay: vfs.Default,
		}
	})

	return nil
}

func openSelfFile() (*os.File, error) {
	return os.Open(exepath.Abs)
}

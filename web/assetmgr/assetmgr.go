// Package assetmgr facilitates the automatic generation and live updation of
// asset cachebusting tags as files are changed.
package assetmgr

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"github.com/hlandau/degoutils/vfs"
	"github.com/hlandau/xlog"
	"github.com/rjeczalik/notify"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var log, Log = xlog.New("web.assetmgr")

var allDigits = regexp.MustCompile(`^[0-9]+$`)

// Asset manager configuration.
type Config struct {
	Path string // Path to assets.
}

// Represents a known asset file.
type file struct {
	mtime    time.Time
	tag      string
	sha256   []byte
	fullpath string
}

func (f *file) ModTime() time.Time {
	return f.mtime
}

func (f *file) Tag() string {
	return f.tag
}

func (f *file) SHA256() []byte {
	return f.sha256
}

func (f *file) FullPath() string {
	return f.fullpath
}

// Asset manager.
type Manager struct {
	cfg        Config
	eventsChan chan notify.EventInfo
	stopChan   chan struct{}
	stopOnce   sync.Once
	filesMutex sync.RWMutex
	files      map[string]*file
}

// Create new asset manager.
func New(cfg Config) (*Manager, error) {
	m := &Manager{
		cfg:        cfg,
		stopChan:   make(chan struct{}),
		eventsChan: make(chan notify.EventInfo, 20),
		files:      make(map[string]*file),
	}

	_, err := os.Stat(m.cfg.Path)
	if err == nil {
		err := notify.Watch(filepath.Join(m.cfg.Path, "..."), m.eventsChan, notify.InCloseWrite)
		if err != nil {
			return nil, err
		}
	}

	go m.notifyLoop()

	return m, nil
}

func (m *Manager) notifyLoop() {
	defer notify.Stop(m.eventsChan)

	for {
		select {
		case ev := <-m.eventsChan:
			m.fileChanged(ev.Path())

		case <-m.stopChan:
			return
		}
	}
}

func (m *Manager) fileChanged(name string) {
	// Ignore filenames beginning with . (vim)
	// Also ignore purely numeric filenames, which show up for some reason.
	fn := filepath.Base(name)
	if fn == "" || fn[0] == '.' || allDigits.MatchString(fn) {
		return
	}

	rname, err := filepath.Rel(m.cfg.Path, name)
	if err != nil {
		log.Errore(err, "relpath changed file")
		return
	}

	err = m.addFile(rname)
	if err != nil {
		log.Errore(err, "couldn't add file: ", rname)
		return
	}
}

func (m *Manager) addFile(path string) error {
	f, err := m.scanFile(path)
	if err != nil {
		return err
	}

	m.filesMutex.Lock()
	defer m.filesMutex.Unlock()
	m.files[path] = f

	log.Debug("file changed: ", path, " ", f.tag)

	return nil
}

func (m *Manager) scanFile(path string) (*file, error) {
	f := &file{
		fullpath: filepath.Join(m.cfg.Path, path),
	}

	fh, err := vfs.Open(f.fullpath)
	if err != nil {
		log.Debuge(err, "cannot find asset with path: ", f.fullpath)
		return f, nil
	}
	defer fh.Close()

	fi, err := fh.Stat()
	if err != nil {
		log.Debuge(err, "stat")
		return f, nil
	}

	f.mtime = fi.ModTime()

	h := sha256.New()
	_, err = io.Copy(h, fh)
	if err != nil {
		log.Debuge(err, "copy")
		return f, nil
	}

	hash := h.Sum(nil)
	f.sha256 = hash
	f.tag = base64.RawURLEncoding.EncodeToString(f.sha256)
	//f.tag = timeToTag(f.mtime)

	return f, nil
}

func timeToTag(t time.Time) string {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(t.Unix()&0xFFFFFFFF))
	return base64.RawURLEncoding.EncodeToString(b)
}

// Represents an asset.
type Info interface {
	// Return asset modification time.
	ModTime() time.Time

	// Return cache invalidating hash tag.
	Tag() string

	// SHA256 hash of data.
	SHA256() []byte

	// Return path to asset.
	FullPath() string
}

// Return info for the asset with the given path.
func (m *Manager) Info(path string) Info {
	m.filesMutex.RLock()
	f, ok := m.files[path]
	m.filesMutex.RUnlock()

	if !ok {
		err := m.addFile(path)
		if err == nil {
			return m.Info(path)
		} else {
			return nil
		}
	}

	if f.tag == "" {
		return nil
	}

	return f
}

// Return cache invalidating hash tag for the asset with the given path.
func (m *Manager) Tag(path string) string {
	i := m.Info(path)
	if i == nil {
		return ""
	}

	return i.Tag()
}

// Return SHA256 of file data or nil if file not found.
func (m *Manager) SHA256(path string) []byte {
	i := m.Info(path)
	if i == nil {
		return nil
	}

	return i.SHA256()
}

// Shut down the asset manager.
func (m *Manager) Close() {
	m.stopOnce.Do(func() {
		close(m.stopChan)
	})
}

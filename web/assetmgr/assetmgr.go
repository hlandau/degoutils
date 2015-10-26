// Package assetmgr facilitates the automatic generation and live updation of
// asset cachebusting tags as files are changed.
package assetmgr

import "github.com/hlandau/xlog"
import "github.com/rjeczalik/notify"
import "path/filepath"
import "regexp"
import "strings"
import "time"
import "os"
import "encoding/binary"
import "encoding/base64"
import "sync"

var log, Log = xlog.New("web.assetmgr")

var allDigits = regexp.MustCompilePOSIX(`^[0-9]+$`)

// Asset manager configuration.
type Config struct {
	Path string // Path to assets.
}

type file struct {
	mtime    time.Time
	tag      string
	fullpath string
}

func (f *file) ModTime() time.Time {
	return f.mtime
}

func (f *file) Tag() string {
	return f.tag
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

	err := notify.Watch(filepath.Join(m.cfg.Path, "..."), m.eventsChan, notify.InCloseWrite)
	if err != nil {
		return nil, err
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
	if len(fn) == 0 || fn[0] == '.' || allDigits.MatchString(fn) {
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
	f := &file{}

	fpath := filepath.Join(m.cfg.Path, path)
	f.fullpath = fpath

	fi, err := os.Stat(fpath)
	if err != nil {
		log.Debug("cannot find asset with path: ", fpath)
		return f, nil
		//return nil, err
	}

	f.mtime = fi.ModTime()
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(f.mtime.Unix()&0xFFFFFFFF))
	f.tag = strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")

	return f, nil
}

// Represents an asset.
type Info interface {
	// Return asset modification time.
	ModTime() time.Time

	// Return cache invalidating hash tag.
	Tag() string

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

// Shut down the asset manager.
func (m *Manager) Close() {
	m.stopOnce.Do(func() {
		close(m.stopChan)
	})
}

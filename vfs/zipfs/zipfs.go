package zipfs

import "archive/zip"
import "github.com/hlandau/degoutils/vfs"
import "github.com/daaku/go.zipexe"
import "io"
import "os"
import "fmt"
import "time"
import "path"
import "strings"

type ReaderAtCloser interface {
	io.Closer
	io.ReaderAt
}

type zipArchive struct {
	rac  ReaderAtCloser
	list zipList
}

// Create a new archive.
func New(rac ReaderAtCloser, size int64) (vfs.Filesystem, error) {
	za := &zipArchive{
		rac: rac,
	}

	// Open ZIP archive.
	var err error
	r, err := zipexe.NewReader(za.rac, size)
	if err != nil {
		return nil, err
	}

	// Create file list.
	za.list = make(zipList, 0, len(r.File))
	for _, f := range r.File {
		if !f.Mode().IsDir() {
			if f.Method != zip.Store || f.CompressedSize64 != f.UncompressedSize64 {
				return nil, fmt.Errorf("zip file contains compressed file, not supported")
			}
		}

		offset, err := f.DataOffset()
		if err != nil {
			return nil, err
		}

		za.list = append(za.list, &zipFile{
			f:      f,
			offset: offset,
		})
	}

	// Sort list of files and directories for lookup purposes.
	za.list.Sort()

	return za, nil
}

// Close archive.
func (za *zipArchive) Close() error {
	return za.rac.Close()
}

// Open file.
func (za *zipArchive) Open(name string) (vfs.File, error) {
	i, exact := za.list.Lookup(name)
	if i < 0 {
		return nil, fmt.Errorf("not found")
	}

	if !exact {
		n := za.list[i].f.Name
		if n[0:len(n)-1] != name {
			return nil, fmt.Errorf("not found")
		}
	}

	f := za.list[i]
	if f.IsDir() {
		return &zipDir{
			list: za.list[i:],
		}, nil
	}

	size := f.f.UncompressedSize64

	return &zipReader{
		SectionReader: io.NewSectionReader(za.rac, f.offset, int64(size)),
		zf:            f,
	}, nil
}

var ErrReadOnly = fmt.Errorf("read only ZIP archive")

func (za *zipArchive) Create(name string) (vfs.File, error) {
	return nil, ErrReadOnly
}

func (za *zipArchive) OpenFile(name string, flag int, perm os.FileMode) (vfs.File, error) {
	return nil, ErrReadOnly
}

func (za *zipArchive) Remove(name string) error {
	return ErrReadOnly
}

func (za *zipArchive) RemoveAll(path string) error {
	return ErrReadOnly
}

func (za *zipArchive) Rename(oldPath, newPath string) error {
	return ErrReadOnly
}

func (za *zipArchive) Mkdir(name string, perm os.FileMode) error {
	return ErrReadOnly
}

func (za *zipArchive) MkdirAll(name string, perm os.FileMode) error {
	return ErrReadOnly
}

func (za *zipArchive) Link(oldPath, newPath string) error {
	return ErrReadOnly
}

func (za *zipArchive) Symlink(oldPath, newPath string) error {
	return ErrReadOnly
}

func (za *zipArchive) Readlink(path string) (string, error) {
	return "", ErrReadOnly
}

func (za *zipArchive) Chmod(name string, mode os.FileMode) error {
	return ErrReadOnly
}

func (za *zipArchive) Chown(name string, uid, gid int) error {
	return ErrReadOnly
}

func (za *zipArchive) Lchown(name string, uid, gid int) error {
	return ErrReadOnly
}

func (za *zipArchive) Truncate(name string, size int64) error {
	return ErrReadOnly
}

func (za *zipArchive) Stat(name string) (os.FileInfo, error) {
	f, err := za.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}

func (za *zipArchive) Lstat(name string) (os.FileInfo, error) {
	return za.Stat(name)
}

// Represents an uncompressed file at a given location in the archive.
type zipFile struct {
	f      *zip.File
	offset int64
}

// zipFile is also its own os.FileInfo.
func (zf *zipFile) Stat() (os.FileInfo, error) {
	return zf, nil
}

func (zf *zipFile) Name() string {
	return path.Base(zf.f.Name)
}

func (zf *zipFile) Size() int64 {
	return int64(zf.f.UncompressedSize64)
}

func (zf *zipFile) Mode() os.FileMode {
	return zf.f.Mode()
}

func (zf *zipFile) IsDir() bool {
	return zf.Mode().IsDir()
}

func (zf *zipFile) ModTime() time.Time {
	return zf.f.ModTime()
}

func (zf *zipFile) Sys() interface{} {
	return zf.f
}

// Stream for reading a file in a ZIP archive.
type zipReader struct {
	*io.SectionReader
	zf *zipFile
}

func (zr *zipReader) Close() error {
	return nil
}

var ErrNotDirectory = fmt.Errorf("not a directory")

// To satisfy http.File.
func (zr *zipReader) Readdir(count int) ([]os.FileInfo, error) {
	return nil, ErrNotDirectory
}

func (zr *zipReader) Readdirnames(count int) ([]string, error) {
	return nil, ErrNotDirectory
}

func (zr *zipReader) Sync() error {
	return ErrReadOnly
}

func (zr *zipReader) Truncate(int64) error {
	return ErrReadOnly
}

func (zr *zipReader) Write([]byte) (int, error) {
	return 0, ErrReadOnly
}

func (zr *zipReader) WriteAt([]byte, int64) (int, error) {
	return 0, ErrReadOnly
}

func (zr *zipReader) Stat() (os.FileInfo, error) {
	return zr.zf.Stat()
}

// Directory.
type zipDir struct {
	list zipList
	i    int
}

func (zr *zipDir) Close() error {
	return nil
}

var ErrDirectory = fmt.Errorf("is a directory")

func (zr *zipDir) Read([]byte) (int, error) {
	return 0, ErrDirectory
}

func (zr *zipDir) ReadAt([]byte, int64) (int, error) {
	return 0, ErrDirectory
}

func (zr *zipDir) Write([]byte) (int, error) {
	return 0, ErrDirectory
}

func (zr *zipDir) WriteAt([]byte, int64) (int, error) {
	return 0, ErrDirectory
}

func (zr *zipDir) Sync() error {
	return ErrDirectory
}

func (zr *zipDir) Truncate(int64) error {
	return ErrDirectory
}

func (zr *zipDir) Readdir(count int) ([]os.FileInfo, error) {
	var fi []os.FileInfo

	p := strings.LastIndexByte(zr.list[0].f.Name, '/')

	var i int
	var f *zipFile
	for i, f = range zr.list[zr.i:] {
		// If the name is not under the directory, we have reached
		// the end of the listing.
		if !strings.HasPrefix(f.f.Name, zr.list[0].f.Name) {
			break
		}

		// Strip trailing slash on subdirectory names.
		name := f.f.Name
		if name[len(name)-1] == '/' {
			name = name[0 : len(name)-1]
		}

		// Inside a subdirectory, skip.
		if strings.LastIndexByte(name, '/') != p {
			continue
		}

		fi = append(fi, f)

		if count > 0 && len(fi) >= count {
			break
		}
	}

	zr.i += i

	if count > 0 && len(fi) == 0 {
		return nil, io.EOF
	}

	return fi, nil
}

func (zr *zipDir) Readdirnames(count int) ([]string, error) {
	fis, err := zr.Readdir(count)
	var names []string
	for _, fi := range fis {
		names = append(names, fi.Name())
	}
	return names, err
}

func (zr *zipDir) Seek(int64, int) (int64, error) {
	return 0, ErrDirectory
}

func (zr *zipDir) Stat() (os.FileInfo, error) {
	return zr.list[0], nil
}

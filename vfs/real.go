package vfs

import "os"
import "path/filepath"

// Creates a new Filesystem which access files under the given real path. The
// path is made absolute and so is not affected by future changes in working
// directory.
func Real(path string) (Filesystem, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	return RealRel(p)
}

// Like Real, but does not absolutize the path, so the files accessed via the
// returned Filesystem will vary depending on the current working directory.
func RealRel(path string) (Filesystem, error) {
	return &real{
		path: path,
	}, nil
}

// Accesses the host OS filesystem.
type real struct {
	path string
}

func ajoin(a, b string) string {
	if filepath.IsAbs(b) {
		return b
	}
	return filepath.Join(a, b)
}

func (r *real) p(name string) string {
	return ajoin(r.path, name)
}

func (r *real) Open(name string) (File, error) {
	return os.Open(r.p(name))
}

func (r *real) Create(name string) (File, error) {
	return os.Create(r.p(name))
}

func (r *real) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	return os.OpenFile(r.p(name), flag, perm)
}

func (r *real) Stat(name string) (os.FileInfo, error) {
	return os.Stat(r.p(name))
}

func (r *real) Lstat(name string) (os.FileInfo, error) {
	return os.Lstat(r.p(name))
}

func (r *real) Remove(name string) error {
	return os.Remove(r.p(name))
}

func (r *real) RemoveAll(name string) error {
	return os.RemoveAll(r.p(name))
}

func (r *real) Rename(oldPath, newPath string) error {
	return os.Rename(r.p(oldPath), r.p(newPath))
}

func (r *real) Link(oldPath, newPath string) error {
	return os.Link(r.p(oldPath), r.p(newPath))
}

func (r *real) Symlink(oldPath, newPath string) error {
	return os.Symlink(r.p(oldPath), r.p(newPath))
}

func (r *real) Readlink(name string) (string, error) {
	return os.Readlink(r.p(name))
}

func (r *real) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(r.p(name), perm)
}

func (r *real) MkdirAll(name string, perm os.FileMode) error {
	return os.MkdirAll(r.p(name), perm)
}

func (r *real) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(r.p(name), mode)
}

func (r *real) Chown(name string, uid, gid int) error {
	return os.Chown(r.p(name), uid, gid)
}

func (r *real) Lchown(name string, uid, gid int) error {
	return os.Lchown(r.p(name), uid, gid)
}

func (r *real) Truncate(name string, size int64) error {
	return os.Truncate(r.p(name), size)
}

func (r *real) Close() error {
	return nil
}

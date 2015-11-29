package vfs

import "os"

type Overlay struct {
	Overlay  Filesystem
	Underlay Filesystem
}

func (r *Overlay) Close() error {
	return nil
}

func (r *Overlay) Open(name string) (File, error) {
	f, err := r.Overlay.Open(name)
	if err == nil {
		return f, nil
	}
	return r.Underlay.Open(name)
}

func (r *Overlay) Create(name string) (File, error) {
	f, err := r.Overlay.Create(name)
	if err == nil {
		return f, nil
	}
	return r.Underlay.Create(name)
}

func (r *Overlay) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	f, err := r.Overlay.Create(name)
	if err == nil {
		return f, nil
	}
	return r.Underlay.OpenFile(name, flag, perm)
}

func (r *Overlay) Stat(name string) (os.FileInfo, error) {
	f, err := r.Overlay.Stat(name)
	if err == nil {
		return f, nil
	}
	return r.Underlay.Stat(name)
}

func (r *Overlay) Lstat(name string) (os.FileInfo, error) {
	f, err := r.Overlay.Lstat(name)
	if err == nil {
		return f, nil
	}
	return r.Underlay.Lstat(name)
}

func (r *Overlay) Remove(name string) error {
	err := r.Overlay.Remove(name)
	if err == nil {
		return nil
	}
	return r.Underlay.Remove(name)
}

func (r *Overlay) RemoveAll(name string) error {
	err := r.Overlay.RemoveAll(name)
	if err == nil {
		return nil
	}
	return r.Underlay.RemoveAll(name)
}

func (r *Overlay) Rename(oldPath, newPath string) error {
	err := r.Overlay.Rename(oldPath, newPath)
	if err == nil {
		return nil
	}
	return r.Underlay.Rename(oldPath, newPath)
}

func (r *Overlay) Link(oldPath, newPath string) error {
	err := r.Overlay.Link(oldPath, newPath)
	if err == nil {
		return nil
	}
	return r.Underlay.Link(oldPath, newPath)
}

func (r *Overlay) Symlink(oldPath, newPath string) error {
	err := r.Overlay.Symlink(oldPath, newPath)
	if err == nil {
		return nil
	}
	return r.Underlay.Symlink(oldPath, newPath)
}

func (r *Overlay) Readlink(name string) (string, error) {
	f, err := r.Overlay.Readlink(name)
	if err == nil {
		return f, nil
	}
	return r.Underlay.Readlink(name)
}

func (r *Overlay) Mkdir(name string, perm os.FileMode) error {
	err := r.Overlay.Mkdir(name, perm)
	if err == nil {
		return nil
	}
	return r.Underlay.Mkdir(name, perm)
}

func (r *Overlay) MkdirAll(name string, perm os.FileMode) error {
	err := r.Overlay.MkdirAll(name, perm)
	if err == nil {
		return nil
	}
	return r.Underlay.MkdirAll(name, perm)
}

func (r *Overlay) Chmod(name string, mode os.FileMode) error {
	err := r.Overlay.Chmod(name, mode)
	if err == nil {
		return nil
	}
	return r.Underlay.Chmod(name, mode)
}

func (r *Overlay) Chown(name string, uid, gid int) error {
	err := r.Overlay.Chown(name, uid, gid)
	if err == nil {
		return nil
	}
	return r.Underlay.Chown(name, uid, gid)
}

func (r *Overlay) Lchown(name string, uid, gid int) error {
	err := r.Overlay.Lchown(name, uid, gid)
	if err == nil {
		return nil
	}
	return r.Underlay.Lchown(name, uid, gid)
}

func (r *Overlay) Truncate(name string, size int64) error {
	err := r.Overlay.Truncate(name, size)
	if err == nil {
		return nil
	}
	return r.Underlay.Truncate(name, size)
}

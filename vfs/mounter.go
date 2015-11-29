package vfs

import "os"
import "fmt"
import "sort"
import "strings"

type mountpoint struct {
	Prefix     string
	Filesystem Filesystem
}

type Mounter struct {
	prefixes []mountpoint
}

type sorter []mountpoint

func (s sorter) Len() int {
	return len(s)
}

func (s sorter) Less(i, j int) bool {
	return s[i].Prefix < s[j].Prefix
}

func (s sorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (r *Mounter) Mount(mountPoint string, fs Filesystem) error {
	r.prefixes = append(r.prefixes, mountpoint{
		Prefix:     mountPoint,
		Filesystem: fs,
	})

	sort.Stable(sorter(r.prefixes))
	return nil
}

func (r *Mounter) Unmount(mountPoint string) {
	var newPrefixes []mountpoint
	for _, p := range r.prefixes {
		if p.Prefix != mountPoint {
			newPrefixes = append(newPrefixes, p)
		}
	}
	r.prefixes = newPrefixes
}

func (r *Mounter) fs(name string) (fs Filesystem, subPath string, err error) {
	i := sort.Search(len(r.prefixes), func(i int) bool {
		return r.prefixes[i].Prefix >= name
	})
	if i <= 0 || i > len(r.prefixes) {
		return nil, name, fmt.Errorf("not found")
	}
	mp := &r.prefixes[i-1]
	if !strings.HasPrefix(name, mp.Prefix) {
		return nil, name, fmt.Errorf("not found")
	}
	return mp.Filesystem, name[len(mp.Prefix)+1:], nil
}

func (r *Mounter) fs2(oldName, newName string) (fs Filesystem, oldSubPath, newSubPath string, err error) {
	oldFS, oldP, err := r.fs(oldName)
	if err != nil {
		return nil, "", "", err
	}

	newFS, newP, err := r.fs(newName)
	if err != nil {
		return nil, "", "", err
	}

	if oldFS != newFS {
		return nil, "", "", fmt.Errorf("cross-filesystem operations not supported")
	}

	return oldFS, oldP, newP, nil
}

func (r *Mounter) Close() error {
	return nil
}

func (r *Mounter) Open(name string) (File, error) {
	fs, p, err := r.fs(name)
	if err != nil {
		return nil, err
	}
	return fs.Open(p)
}

func (r *Mounter) Create(name string) (File, error) {
	fs, p, err := r.fs(name)
	if err != nil {
		return nil, err
	}
	return fs.Create(p)
}

func (r *Mounter) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	fs, p, err := r.fs(name)
	if err != nil {
		return nil, err
	}
	return fs.OpenFile(p, flag, perm)
}

func (r *Mounter) Stat(name string) (os.FileInfo, error) {
	fs, p, err := r.fs(name)
	if err != nil {
		return nil, err
	}
	return fs.Stat(p)
}

func (r *Mounter) Lstat(name string) (os.FileInfo, error) {
	fs, p, err := r.fs(name)
	if err != nil {
		return nil, err
	}
	return fs.Lstat(p)
}

func (r *Mounter) Remove(name string) error {
	fs, p, err := r.fs(name)
	if err != nil {
		return err
	}
	return fs.Remove(p)
}

func (r *Mounter) RemoveAll(name string) error {
	fs, p, err := r.fs(name)
	if err != nil {
		return err
	}
	return fs.RemoveAll(p)
}

func (r *Mounter) Rename(oldPath, newPath string) error {
	fs, oldP, newP, err := r.fs2(oldPath, newPath)
	if err != nil {
		return err
	}
	return fs.Rename(oldP, newP)
}

func (r *Mounter) Link(oldPath, newPath string) error {
	fs, oldP, newP, err := r.fs2(oldPath, newPath)
	if err != nil {
		return err
	}
	return fs.Link(oldP, newP)
}

func (r *Mounter) Symlink(oldPath, newPath string) error {
	fs, oldP, newP, err := r.fs2(oldPath, newPath)
	if err != nil {
		return err
	}
	return fs.Symlink(oldP, newP)
}

func (r *Mounter) Readlink(name string) (string, error) {
	fs, p, err := r.fs(name)
	if err != nil {
		return "", err
	}
	return fs.Readlink(p)
}

func (r *Mounter) Mkdir(name string, perm os.FileMode) error {
	fs, p, err := r.fs(name)
	if err != nil {
		return err
	}
	return fs.Mkdir(p, perm)
}

func (r *Mounter) MkdirAll(name string, perm os.FileMode) error {
	fs, p, err := r.fs(name)
	if err != nil {
		return err
	}
	return fs.MkdirAll(p, perm)
}

func (r *Mounter) Chmod(name string, mode os.FileMode) error {
	fs, p, err := r.fs(name)
	if err != nil {
		return err
	}
	return fs.Chmod(p, mode)
}

func (r *Mounter) Chown(name string, uid, gid int) error {
	fs, p, err := r.fs(name)
	if err != nil {
		return err
	}
	return fs.Chown(p, uid, gid)
}

func (r *Mounter) Lchown(name string, uid, gid int) error {
	fs, p, err := r.fs(name)
	if err != nil {
		return err
	}
	return fs.Lchown(p, uid, gid)
}

func (r *Mounter) Truncate(name string, size int64) error {
	fs, p, err := r.fs(name)
	if err != nil {
		return err
	}
	return fs.Truncate(p, size)
}

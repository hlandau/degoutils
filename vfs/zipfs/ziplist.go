package zipfs

import "sort"
import "strings"

// Sorted list of files, used for binary search.
type zipList []*zipFile

// sort.Interface.
func (zl zipList) Len() int           { return len(zl) }
func (zl zipList) Less(i, j int) bool { return zl[i].f.Name < zl[j].f.Name }
func (zl zipList) Swap(i, j int)      { zl[i], zl[j] = zl[j], zl[i] }

// Returns the smallest index of an entry with an exact match for "name",
// or an inexact match starting with "name/". If there is no such entry,
// returns (-1, false).
func (zl zipList) Lookup(name string) (idx int, exact bool) {
	// Look for exact match.
	// "name" comes before "name/" in zl.
	i := sort.Search(len(zl), func(i int) bool {
		return name <= zl[i].f.Name
	})

	if i >= len(zl) {
		return -1, false
	}

	if zl[i].f.Name == name {
		return i, true
	}

	// Look for inexact match in zl[i:].
	zl = zl[i:]
	name += "/"
	j := sort.Search(len(zl), func(i int) bool {
		return name <= zl[i].f.Name
	})

	if j >= len(zl) {
		return -1, false
	}

	// 0 <= j < len(zl)
	if strings.HasPrefix(zl[j].f.Name, name) {
		return i + j, false
	}

	return -1, false
}

func (zl zipList) Sort() {
	sort.Sort(zl)
}

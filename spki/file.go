package spki

import "io"
import "fmt"
import "github.com/hlandau/sx"

// Represents a file.
type FileInfo struct {
	Filename string
	Length   int64
	HashType HashType
	Hash     []byte
}

func (fi *FileInfo) Form() []interface{} {
	return []interface{}{
		"file-representation",
		[]interface{}{"filename", fi.Filename},
		[]interface{}{"length", fi.Length},
		fi.HashType.Form(fi.Hash),
	}
}

// Reads all data from the reader and returns FileInfo.  The filename will be
// blank; you should fill it in yourself.
func HashFile(r io.Reader) (*FileInfo, error) {
	ht := PreferredHashType

	h := ht.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		Length:   n,
		HashType: ht,
		Hash:     h.Sum(nil),
	}, nil
}

var ErrMalformedFileInfo = fmt.Errorf("not a well-formed file representation S-expression structure")

func LoadFileInfo(x []interface{}) (*FileInfo, error) {
	fi := &FileInfo{}

	r := sx.Q1bsyt(x, "file-representation")
	if r == nil {
		return nil, ErrMalformedFileInfo
	}

	// length
	lv := sx.Q1bsyt(r, "length")
	if lv == nil || len(lv) == 0 {
		return nil, ErrMalformedFileInfo
	}
	switch lvi := lv[0].(type) {
	case int:
		fi.Length = int64(lvi)
	case int64:
		fi.Length = lvi
	default:
		return nil, ErrMalformedFileInfo
	}

	// hash
	ht, h, err := LoadHash(r)
	if err != nil {
		return nil, ErrMalformedFileInfo
	}
	fi.Hash = h
	fi.HashType = ht

	// filename (optional)
	fnv := sx.Q1bsyt(r, "filename")
	if fnv != nil {
		if len(fnv) == 0 {
			return nil, ErrMalformedFileInfo
		}
		if fnvs, ok := fnv[0].(string); ok {
			fi.Filename = fnvs
		} else {
			return nil, ErrMalformedFileInfo
		}
	}

	return fi, nil
}

package system

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// NativeExecutable recognizes native file headers without executing/loading the
// file. An unknown text shim cannot supply a conclusive interpreter resolution.
func NativeExecutable(path, goos string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	var header [4]byte
	if _, err = io.ReadFull(f, header[:]); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return false, nil
		}
		return false, err
	}
	switch goos {
	case "windows":
		if header[0] != 'M' || header[1] != 'Z' {
			return false, nil
		}
		if _, err := f.ReadAt(header[:], 0x3c); err != nil {
			return false, nil
		}
		offset := binary.LittleEndian.Uint32(header[:])
		if offset > 4<<20 {
			return false, nil
		}
		if _, err := f.ReadAt(header[:], int64(offset)); err != nil {
			return false, nil
		}
		return string(header[:]) == "PE\x00\x00", nil
	case "linux":
		return string(header[:]) == "\x7fELF", nil
	case "darwin":
		magic := binary.BigEndian.Uint32(header[:])
		return magic == 0xfeedface || magic == 0xfeedfacf || magic == 0xcefaedfe || magic == 0xcffaedfe || magic == 0xcafebabe || magic == 0xbebafeca || magic == 0xcafebabf || magic == 0xbfbafeca, nil
	}
	return false, nil
}

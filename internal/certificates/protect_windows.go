//go:build windows

package certificates

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const cryptprotectUIForbidden = 0x1

func protect(data []byte) ([]byte, error)   { return cryptData(data, true) }
func unprotect(data []byte) ([]byte, error) { return cryptData(data, false) }

func cryptData(data []byte, encrypt bool) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	var err error
	if encrypt {
		err = windows.CryptProtectData(&in, nil, nil, 0, nil, cryptprotectUIForbidden, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, cryptprotectUIForbidden, &out)
	}
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}

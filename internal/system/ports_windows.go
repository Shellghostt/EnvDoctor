//go:build windows

package system

import (
	"context"
	"encoding/binary"
	"fmt"
	"golang.org/x/sys/windows"
	"path/filepath"
	"unsafe"
)

// GetExtendedTcpTable reports numeric TCP states independent of OS language.
// Process names use a query-only handle; access-denied owners retain their PID.
func ListeningPorts(ctx context.Context) ([]Socket, error) {
	proc := windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")
	out := []Socket{}
	for _, family := range []uintptr{2, 23} {
		var size uint32
		code, _, _ := proc.Call(0, uintptr(unsafe.Pointer(&size)), 0, family, 3, 0)
		if code != 0 && code != 122 {
			return nil, fmt.Errorf("TCP table query failed (%d)", code)
		}
		var data []byte
		for attempt := 0; attempt < 3; attempt++ {
			if size < 4 || size > 16<<20 {
				return nil, fmt.Errorf("invalid TCP table size")
			}
			data = make([]byte, size)
			code, _, _ = proc.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&size)), 0, family, 3, 0)
			if code != 122 {
				break
			}
		}
		if code != 0 {
			return nil, fmt.Errorf("TCP table read failed (%d)", code)
		}
		count := int(binary.LittleEndian.Uint32(data))
		rowSize := 24
		if family == 23 {
			rowSize = 56
		}
		if count > (len(data)-4)/rowSize {
			return nil, fmt.Errorf("truncated TCP table")
		}
		for i := 0; i < count; i++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			row := data[4+i*rowSize:]
			stateOffset, portOffset, pidOffset := 0, 8, 20
			if family == 23 {
				stateOffset, portOffset, pidOffset = 48, 20, 52
			}
			if binary.LittleEndian.Uint32(row[stateOffset:]) != 2 {
				continue
			}
			port := int(binary.BigEndian.Uint16(row[portOffset:]))
			pid := binary.LittleEndian.Uint32(row[pidOffset:])
			name := ""
			handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
			if err == nil {
				buf := make([]uint16, 32768)
				n := uint32(len(buf))
				if windows.QueryFullProcessImageName(handle, 0, &buf[0], &n) == nil {
					name = filepath.Base(windows.UTF16ToString(buf[:n]))
				}
				_ = windows.CloseHandle(handle)
			}
			out = append(out, Socket{Port: port, PID: int(pid), Process: name})
		}
	}
	return out, nil
}

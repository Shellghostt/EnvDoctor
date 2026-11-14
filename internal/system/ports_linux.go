//go:build linux

package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ListeningPorts(ctx context.Context) ([]Socket, error) {
	sockets := []Socket{}
	inodes := map[string][]int{}
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := ReadSmallFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, line := range strings.Split(string(data), "\n") {
			f := strings.Fields(line)
			if len(f) < 10 || f[3] != "0A" {
				continue
			}
			_, hex, ok := strings.Cut(f[1], ":")
			if !ok {
				continue
			}
			port, err := strconv.ParseInt(hex, 16, 32)
			if err != nil {
				continue
			}
			inodes[f[9]] = append(inodes[f[9]], len(sockets))
			sockets = append(sockets, Socket{Port: int(port)})
		}
	}
	processes, _ := os.ReadDir("/proc")
	for _, p := range processes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pid, err := strconv.Atoi(p.Name())
		if err != nil {
			continue
		}
		fds, _ := os.ReadDir(filepath.Join("/proc", p.Name(), "fd"))
		for _, fd := range fds {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			target, err := os.Readlink(filepath.Join("/proc", p.Name(), "fd", fd.Name()))
			if err != nil {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
			if positions := inodes[inode]; len(positions) > 0 {
				name, _ := ReadSmallFile(fmt.Sprintf("/proc/%d/comm", pid))
				for _, i := range positions {
					if sockets[i].PID == 0 || pid < sockets[i].PID {
						sockets[i].PID = pid
						sockets[i].Process = strings.TrimSpace(string(name))
					}
				}
			}
		}
	}
	return sockets, nil
}

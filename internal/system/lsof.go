package system

import (
	"strconv"
	"strings"
)

// lsof exits 1 when its selection matches no sockets. Launch failures and
// other process errors mean inspection is degraded, not that the port is free.
func parseLSOF(out []byte, err error) ([]Socket, error) {
	if err != nil && (len(out) != 0 || !ExitedWith(err, 1)) {
		return nil, err
	}
	sockets := []Socket{}
	pid, name := 0, ""
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'c':
			name = line[1:]
		case 'n':
			if at := strings.LastIndex(line, ":"); at >= 0 {
				if port, err := strconv.Atoi(line[at+1:]); err == nil && port > 0 && port <= 65535 {
					sockets = append(sockets, Socket{Port: port, PID: pid, Process: name})
				}
			}
		}
	}
	return sockets, nil
}

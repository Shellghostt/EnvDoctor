//go:build darwin

package system

import (
	"context"
)

func ListeningPorts(ctx context.Context) ([]Socket, error) {
	out, err := RunTool(ctx, InstalledTool("lsof"), "-nP", "-iTCP", "-sTCP:LISTEN", "-Fpcn")
	return parseLSOF(out, err)
}

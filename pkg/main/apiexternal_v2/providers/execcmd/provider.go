// Package execcmd "sends" notifications by running a local program.
package execcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

// Timeout bounds one run of the program.
const Timeout = 60 * time.Second

var errNotExecutable = errors.New("exec: the command is not an executable file")

// Provider runs one program per notification. The program is started
// directly, not through a shell, with the title and message as its two
// arguments and as GMD_TITLE / GMD_MESSAGE in its environment, so release
// names can't inject shell syntax.
type Provider struct {
	command string
}

// NewProvider returns a provider for the program at command (a path).
func NewProvider(command string) *Provider {
	return &Provider{command: command}
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationExec
}

func (*Provider) GetProviderName() string { return "exec" }

// SendNotification runs the program and waits for it, up to Timeout. A
// non-zero exit is an error carrying the program's output.
func (p *Provider) SendNotification(
	ctx context.Context,
	req apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.command, req.Title, req.Message)
	cmd.Env = append(os.Environ(), "GMD_TITLE="+req.Title, "GMD_MESSAGE="+req.Message)

	var out bytes.Buffer

	cmd.Stdout, cmd.Stderr = &out, &out

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(out.String())
		if len(msg) > 500 {
			msg = msg[:500]
		}

		return nil, fmt.Errorf("exec %s: %w: %s", p.command, err, msg)
	}

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationExec), nil
}

// TestConnection checks that the command is an executable file, without
// running it.
func (p *Provider) TestConnection(context.Context) error {
	fi, err := os.Stat(p.command)
	if err != nil {
		return err
	}

	if fi.IsDir() || fi.Mode().Perm()&0o111 == 0 {
		return errNotExecutable
	}

	return nil
}

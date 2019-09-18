package kubernetes

import (
	"strings"
	"os"

	"github.com/policygenius/monday/pkg/ui"
)

var(
	debugMode = len(os.Getenv("BIFROST_ENABLE_DEBUG")) > 0
)
type Logstreamer struct {
	podName string
	view    ui.ViewInterface
}

func NewLogstreamer(view ui.ViewInterface, podName string) *Logstreamer {
	return &Logstreamer{
		podName: podName,
		view:    view,
	}
}

func (l *Logstreamer) Write(b []byte) (int, error) {
	if debugMode {
		line := string(b)
		strings.TrimSuffix(line, "\n")

		l.view.Writef("%s %s", l.podName, line)

		return 0, nil
	} else {
		return 0, nil
	}

}

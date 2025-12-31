package desktop

import (
	"fmt"
	"sync/atomic"
)

const UserAgentPrefix string = "Corpus-Desktop"

var (
	appPorts = []int{
		37427,
		37428,
		37429,
		37430,
		54392,
		54920,
	}
)

var portIndex atomic.Int32

func NextPort() (int, int) {
	currentPortIndex := portIndex.Load()
	defer func() {
		nextPort := (int(currentPortIndex) + 1) % len(appPorts)
		portIndex.Store(int32(nextPort))
	}()

	return appPorts[currentPortIndex], len(appPorts)
}

func Origins() []string {
	origins := make([]string, 0, len(appPorts))
	for _, p := range appPorts {
		origins = append(origins, fmt.Sprintf("http://127.0.0.1:%d", p))
	}
	return origins
}

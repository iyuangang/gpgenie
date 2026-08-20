//go:build !windows || (!amd64 && !arm64)

package vanity

import (
	"context"
	"fmt"
	"sync/atomic"
)

func listOpenCLDeviceRefs() ([]openCLDeviceRef, error) {
	return nil, fmt.Errorf("the OpenCL backend currently supports Windows amd64/arm64")
}

func searchOpenCLWorker(context.Context, SearchConfig, openCLDeviceRef, *atomic.Uint64, *atomic.Uint64, *atomic.Int32, chan<- Candidate) error {
	return fmt.Errorf("the OpenCL backend currently supports Windows amd64/arm64")
}

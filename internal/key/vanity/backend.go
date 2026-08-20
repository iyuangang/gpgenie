package vanity

import "fmt"

const (
	defaultGPUKeyBatch  = 256
	defaultGPUWorkItems = uint64(64 * 1024 * 1024)
	maxGPUKeyBatch      = 65536
	maxGPUWorkItems     = uint64((1 << 27) - 1)
)

// OpenCLDevice describes one GPU exposed by the system OpenCL ICD. Index is
// stable only for the current driver/device enumeration and is intended for
// the --gpu-devices option.
type OpenCLDevice struct {
	Index             int
	Platform          string
	Vendor            string
	Name              string
	DriverVersion     string
	OpenCLVersion     string
	ComputeUnits      uint32
	GlobalMemoryBytes uint64
	MaxWorkGroupSize  uint64
}

type openCLDeviceRef struct {
	Info     OpenCLDevice
	Platform uintptr
	Device   uintptr
}

func (b Backend) Validate() error {
	switch b {
	case BackendCPU, BackendOpenCL, BackendHybrid, BackendAuto:
		return nil
	default:
		return fmt.Errorf("backend must be %q, %q, %q, or %q", BackendCPU, BackendOpenCL, BackendHybrid, BackendAuto)
	}
}

// ListOpenCLDevices returns all GPU devices provided by installed OpenCL
// drivers. Loading is dynamic, so building gpgenie never requires an OpenCL
// SDK or CUDA toolkit.
func ListOpenCLDevices() ([]OpenCLDevice, error) {
	refs, err := listOpenCLDeviceRefs()
	if err != nil {
		return nil, err
	}
	devices := make([]OpenCLDevice, len(refs))
	for i := range refs {
		devices[i] = refs[i].Info
	}
	return devices, nil
}

func ResolveBackend(requested Backend, selected []int) (Backend, []OpenCLDevice, error) {
	effective, refs, err := resolveBackendRefs(requested, selected)
	if err != nil {
		return "", nil, err
	}
	devices := make([]OpenCLDevice, len(refs))
	for i := range refs {
		devices[i] = refs[i].Info
	}
	return effective, devices, nil
}

func resolveBackendRefs(requested Backend, selected []int) (Backend, []openCLDeviceRef, error) {
	if requested == "" {
		requested = BackendCPU
	}
	if err := requested.Validate(); err != nil {
		return "", nil, err
	}
	if requested == BackendCPU {
		return BackendCPU, nil, nil
	}

	devices, err := listOpenCLDeviceRefs()
	if err != nil {
		if requested == BackendAuto {
			return BackendCPU, nil, nil
		}
		return "", nil, err
	}
	devices, err = selectOpenCLDevices(devices, selected)
	if err != nil {
		return "", nil, err
	}
	if len(devices) == 0 {
		if requested == BackendAuto {
			return BackendCPU, nil, nil
		}
		return "", nil, fmt.Errorf("no OpenCL GPU devices found; install the GPU vendor driver or use --backend cpu")
	}
	if requested == BackendAuto {
		requested = BackendOpenCL
	}
	return requested, devices, nil
}

func selectOpenCLDevices(available []openCLDeviceRef, selected []int) ([]openCLDeviceRef, error) {
	if len(selected) == 0 {
		return available, nil
	}
	seen := make(map[int]bool, len(selected))
	result := make([]openCLDeviceRef, 0, len(selected))
	for _, index := range selected {
		if index < 0 || index >= len(available) {
			return nil, fmt.Errorf("OpenCL GPU index %d is out of range (found %d GPU devices)", index, len(available))
		}
		if seen[index] {
			continue
		}
		seen[index] = true
		result = append(result, available[index])
	}
	return result, nil
}

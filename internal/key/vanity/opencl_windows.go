//go:build windows && (amd64 || arm64)

package vanity

import (
	"context"
	"errors"
	"fmt"
	"math/bits"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"golang.org/x/sys/windows"
)

const (
	clSuccess              = int32(0)
	clDeviceNotFound       = int32(-1)
	clDeviceTypeGPU        = uintptr(1 << 2)
	clPlatformName         = uint32(0x0902)
	clDeviceMaxComputeUnit = uint32(0x1002)
	clDeviceMaxWorkGroup   = uint32(0x1004)
	clDeviceGlobalMemSize  = uint32(0x101F)
	clDeviceName           = uint32(0x102B)
	clDeviceVendor         = uint32(0x102C)
	clDriverVersion        = uint32(0x102D)
	clDeviceVersion        = uint32(0x102F)
	clMemReadWrite         = uintptr(1 << 0)
	clMemReadOnly          = uintptr(1 << 2)
	clProgramBuildLog      = uint32(0x1183)
	clKernelWorkGroupSize  = uint32(0x11B0)
	clTrue                 = uintptr(1)
	openCLHitCapacity      = 4096
	openCLResultWords      = 2 + openCLHitCapacity
	openCLResultIndexBits  = 27
)

type openCLAPI struct {
	dll                     *windows.LazyDLL
	getPlatformIDs          *windows.LazyProc
	getPlatformInfo         *windows.LazyProc
	getDeviceIDs            *windows.LazyProc
	getDeviceInfo           *windows.LazyProc
	createContext           *windows.LazyProc
	createCommandQueue      *windows.LazyProc
	createProgramWithSource *windows.LazyProc
	buildProgram            *windows.LazyProc
	getProgramBuildInfo     *windows.LazyProc
	createKernel            *windows.LazyProc
	getKernelWorkGroupInfo  *windows.LazyProc
	createBuffer            *windows.LazyProc
	setKernelArg            *windows.LazyProc
	enqueueWriteBuffer      *windows.LazyProc
	enqueueNDRangeKernel    *windows.LazyProc
	enqueueReadBuffer       *windows.LazyProc
	finish                  *windows.LazyProc
	releaseMemObject        *windows.LazyProc
	releaseKernel           *windows.LazyProc
	releaseProgram          *windows.LazyProc
	releaseCommandQueue     *windows.LazyProc
	releaseContext          *windows.LazyProc
}

var openCLLoader struct {
	sync.Once
	api *openCLAPI
	err error
}

func loadOpenCL() (*openCLAPI, error) {
	openCLLoader.Do(func() {
		dll := windows.NewLazySystemDLL("OpenCL.dll")
		api := &openCLAPI{
			dll:                     dll,
			getPlatformIDs:          dll.NewProc("clGetPlatformIDs"),
			getPlatformInfo:         dll.NewProc("clGetPlatformInfo"),
			getDeviceIDs:            dll.NewProc("clGetDeviceIDs"),
			getDeviceInfo:           dll.NewProc("clGetDeviceInfo"),
			createContext:           dll.NewProc("clCreateContext"),
			createCommandQueue:      dll.NewProc("clCreateCommandQueue"),
			createProgramWithSource: dll.NewProc("clCreateProgramWithSource"),
			buildProgram:            dll.NewProc("clBuildProgram"),
			getProgramBuildInfo:     dll.NewProc("clGetProgramBuildInfo"),
			createKernel:            dll.NewProc("clCreateKernel"),
			getKernelWorkGroupInfo:  dll.NewProc("clGetKernelWorkGroupInfo"),
			createBuffer:            dll.NewProc("clCreateBuffer"),
			setKernelArg:            dll.NewProc("clSetKernelArg"),
			enqueueWriteBuffer:      dll.NewProc("clEnqueueWriteBuffer"),
			enqueueNDRangeKernel:    dll.NewProc("clEnqueueNDRangeKernel"),
			enqueueReadBuffer:       dll.NewProc("clEnqueueReadBuffer"),
			finish:                  dll.NewProc("clFinish"),
			releaseMemObject:        dll.NewProc("clReleaseMemObject"),
			releaseKernel:           dll.NewProc("clReleaseKernel"),
			releaseProgram:          dll.NewProc("clReleaseProgram"),
			releaseCommandQueue:     dll.NewProc("clReleaseCommandQueue"),
			releaseContext:          dll.NewProc("clReleaseContext"),
		}
		if err := dll.Load(); err != nil {
			openCLLoader.err = fmt.Errorf("load OpenCL.dll: %w", err)
			return
		}
		for name, proc := range map[string]*windows.LazyProc{
			"clGetPlatformIDs": api.getPlatformIDs, "clGetPlatformInfo": api.getPlatformInfo,
			"clGetDeviceIDs": api.getDeviceIDs, "clGetDeviceInfo": api.getDeviceInfo,
			"clCreateContext": api.createContext, "clCreateCommandQueue": api.createCommandQueue,
			"clCreateProgramWithSource": api.createProgramWithSource, "clBuildProgram": api.buildProgram,
			"clGetProgramBuildInfo": api.getProgramBuildInfo, "clCreateKernel": api.createKernel,
			"clCreateBuffer": api.createBuffer, "clSetKernelArg": api.setKernelArg,
			"clGetKernelWorkGroupInfo": api.getKernelWorkGroupInfo,
			"clEnqueueWriteBuffer":     api.enqueueWriteBuffer, "clEnqueueNDRangeKernel": api.enqueueNDRangeKernel,
			"clEnqueueReadBuffer": api.enqueueReadBuffer, "clFinish": api.finish,
			"clReleaseMemObject": api.releaseMemObject, "clReleaseKernel": api.releaseKernel,
			"clReleaseProgram": api.releaseProgram, "clReleaseCommandQueue": api.releaseCommandQueue,
			"clReleaseContext": api.releaseContext,
		} {
			if err := proc.Find(); err != nil {
				openCLLoader.err = fmt.Errorf("load OpenCL symbol %s: %w", name, err)
				return
			}
		}
		openCLLoader.api = api
	})
	return openCLLoader.api, openCLLoader.err
}

func clCode(proc *windows.LazyProc, args ...uintptr) int32 {
	result, _, _ := proc.Call(args...)
	return int32(result)
}

func clCheck(operation string, code int32) error {
	if code == clSuccess {
		return nil
	}
	return fmt.Errorf("%s failed with OpenCL error %d", operation, code)
}

func listOpenCLDeviceRefs() ([]openCLDeviceRef, error) {
	api, err := loadOpenCL()
	if err != nil {
		return nil, err
	}
	var platformCount uint32
	if err := clCheck("clGetPlatformIDs", clCode(api.getPlatformIDs, 0, 0, uintptr(unsafe.Pointer(&platformCount)))); err != nil {
		return nil, err
	}
	if platformCount == 0 {
		return nil, nil
	}
	platforms := make([]uintptr, platformCount)
	if err := clCheck("clGetPlatformIDs", clCode(api.getPlatformIDs, uintptr(platformCount), slicePointer(platforms), 0)); err != nil {
		return nil, err
	}

	var result []openCLDeviceRef
	for _, platform := range platforms {
		platformName, _ := clInfoString(api.getPlatformInfo, platform, clPlatformName)
		var deviceCount uint32
		code := clCode(api.getDeviceIDs, platform, clDeviceTypeGPU, 0, 0, uintptr(unsafe.Pointer(&deviceCount)))
		if code == clDeviceNotFound {
			continue
		}
		if err := clCheck("clGetDeviceIDs", code); err != nil {
			return nil, err
		}
		if deviceCount == 0 {
			continue
		}
		devices := make([]uintptr, deviceCount)
		if err := clCheck("clGetDeviceIDs", clCode(api.getDeviceIDs, platform, clDeviceTypeGPU, uintptr(deviceCount), slicePointer(devices), 0)); err != nil {
			return nil, err
		}
		for _, device := range devices {
			name, _ := clInfoString(api.getDeviceInfo, device, clDeviceName)
			vendor, _ := clInfoString(api.getDeviceInfo, device, clDeviceVendor)
			driver, _ := clInfoString(api.getDeviceInfo, device, clDriverVersion)
			version, _ := clInfoString(api.getDeviceInfo, device, clDeviceVersion)
			computeUnits, _ := clInfoUint32(api.getDeviceInfo, device, clDeviceMaxComputeUnit)
			globalMemory, _ := clInfoUint64(api.getDeviceInfo, device, clDeviceGlobalMemSize)
			maxWorkGroup, _ := clInfoUintptr(api.getDeviceInfo, device, clDeviceMaxWorkGroup)
			result = append(result, openCLDeviceRef{
				Info: OpenCLDevice{
					Index: len(result), Platform: platformName, Vendor: vendor, Name: name,
					DriverVersion: driver, OpenCLVersion: version, ComputeUnits: computeUnits,
					GlobalMemoryBytes: globalMemory, MaxWorkGroupSize: uint64(maxWorkGroup),
				},
				Platform: platform,
				Device:   device,
			})
		}
	}
	runtime.KeepAlive(platforms)
	return result, nil
}

func slicePointer[T any](values []T) uintptr {
	if len(values) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&values[0]))
}

func clInfoString(proc *windows.LazyProc, object uintptr, parameter uint32) (string, error) {
	var size uintptr
	if err := clCheck("OpenCL info size", clCode(proc, object, uintptr(parameter), 0, 0, uintptr(unsafe.Pointer(&size)))); err != nil {
		return "", err
	}
	if size == 0 {
		return "", nil
	}
	buffer := make([]byte, size)
	if err := clCheck("OpenCL info", clCode(proc, object, uintptr(parameter), size, slicePointer(buffer), 0)); err != nil {
		return "", err
	}
	return strings.TrimRight(string(buffer), "\x00"), nil
}

func clInfoUint32(proc *windows.LazyProc, object uintptr, parameter uint32) (uint32, error) {
	var value uint32
	err := clCheck("OpenCL uint info", clCode(proc, object, uintptr(parameter), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&value)), 0))
	return value, err
}

func clInfoUint64(proc *windows.LazyProc, object uintptr, parameter uint32) (uint64, error) {
	var value uint64
	err := clCheck("OpenCL uint64 info", clCode(proc, object, uintptr(parameter), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&value)), 0))
	return value, err
}

func clInfoUintptr(proc *windows.LazyProc, object uintptr, parameter uint32) (uintptr, error) {
	var value uintptr
	err := clCheck("OpenCL size info", clCode(proc, object, uintptr(parameter), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&value)), 0))
	return value, err
}

type openCLExecutor struct {
	api       *openCLAPI
	device    openCLDeviceRef
	context   uintptr
	queue     uintptr
	program   uintptr
	kernel    uintptr
	blocks    uintptr
	result    uintptr
	blockSize uintptr
	localSize uintptr
}

func newOpenCLExecutor(device openCLDeviceRef, keyBatch int) (*openCLExecutor, error) {
	return newOpenCLExecutorSource(device, keyBatch, openCLVanityKernel)
}

func newOpenCLExecutorSource(device openCLDeviceRef, keyBatch int, kernelSource string) (*openCLExecutor, error) {
	api, err := loadOpenCL()
	if err != nil {
		return nil, err
	}
	if keyBatch <= 0 {
		keyBatch = defaultGPUKeyBatch
	}
	executor := &openCLExecutor{api: api, device: device, blockSize: uintptr(keyBatch * 16 * 4)}
	cleanupOnError := func(err error) (*openCLExecutor, error) {
		executor.Close()
		return nil, err
	}

	var code int32
	executor.context, _, _ = api.createContext.Call(0, 1, uintptr(unsafe.Pointer(&device.Device)), 0, 0, uintptr(unsafe.Pointer(&code)))
	if code != clSuccess || executor.context == 0 {
		return cleanupOnError(fmt.Errorf("clCreateContext failed with OpenCL error %d", code))
	}
	executor.queue, _, _ = api.createCommandQueue.Call(executor.context, device.Device, 0, uintptr(unsafe.Pointer(&code)))
	if code != clSuccess || executor.queue == 0 {
		return cleanupOnError(fmt.Errorf("clCreateCommandQueue failed with OpenCL error %d", code))
	}

	source := append([]byte(kernelSource), 0)
	sourcePointer := slicePointer(source)
	executor.program, _, _ = api.createProgramWithSource.Call(executor.context, 1, uintptr(unsafe.Pointer(&sourcePointer)), 0, uintptr(unsafe.Pointer(&code)))
	runtime.KeepAlive(source)
	if code != clSuccess || executor.program == 0 {
		return cleanupOnError(fmt.Errorf("clCreateProgramWithSource failed with OpenCL error %d", code))
	}
	code = clCode(api.buildProgram, executor.program, 1, uintptr(unsafe.Pointer(&device.Device)), 0, 0, 0)
	if code != clSuccess {
		log, _ := executor.buildLog()
		return cleanupOnError(fmt.Errorf("clBuildProgram failed with OpenCL error %d: %s", code, strings.TrimSpace(log)))
	}
	kernelName := append([]byte("vanity_sha1"), 0)
	executor.kernel, _, _ = api.createKernel.Call(executor.program, slicePointer(kernelName), uintptr(unsafe.Pointer(&code)))
	runtime.KeepAlive(kernelName)
	if code != clSuccess || executor.kernel == 0 {
		return cleanupOnError(fmt.Errorf("clCreateKernel failed with OpenCL error %d", code))
	}
	executor.blocks, _, _ = api.createBuffer.Call(executor.context, clMemReadOnly, executor.blockSize, 0, uintptr(unsafe.Pointer(&code)))
	if code != clSuccess || executor.blocks == 0 {
		return cleanupOnError(fmt.Errorf("create OpenCL input buffer failed with error %d", code))
	}
	executor.result, _, _ = api.createBuffer.Call(executor.context, clMemReadWrite, uintptr(openCLResultWords*4), 0, uintptr(unsafe.Pointer(&code)))
	if code != clSuccess || executor.result == 0 {
		return cleanupOnError(fmt.Errorf("create OpenCL result buffer failed with error %d", code))
	}
	executor.localSize = uintptr(256)
	var kernelMaximum uintptr
	if err := clCheck("clGetKernelWorkGroupInfo", clCode(api.getKernelWorkGroupInfo,
		executor.kernel, device.Device, uintptr(clKernelWorkGroupSize), unsafe.Sizeof(kernelMaximum),
		uintptr(unsafe.Pointer(&kernelMaximum)), 0)); err != nil {
		return cleanupOnError(err)
	}
	if kernelMaximum > 0 && kernelMaximum < executor.localSize {
		executor.localSize = highestPowerOfTwo(kernelMaximum)
	}
	if maximum := uintptr(device.Info.MaxWorkGroupSize); maximum > 0 && maximum < executor.localSize {
		executor.localSize = highestPowerOfTwo(maximum)
	}
	if executor.localSize == 0 {
		executor.localSize = 1
	}
	if err := executor.setMemoryArg(0, executor.blocks); err != nil {
		return cleanupOnError(err)
	}
	if err := executor.setMemoryArg(8, executor.result); err != nil {
		return cleanupOnError(err)
	}
	return executor, nil
}

func (e *openCLExecutor) buildLog() (string, error) {
	var size uintptr
	if err := clCheck("clGetProgramBuildInfo", clCode(e.api.getProgramBuildInfo, e.program, e.device.Device, uintptr(clProgramBuildLog), 0, 0, uintptr(unsafe.Pointer(&size)))); err != nil {
		return "", err
	}
	buffer := make([]byte, size)
	if size > 0 {
		if err := clCheck("clGetProgramBuildInfo", clCode(e.api.getProgramBuildInfo, e.program, e.device.Device, uintptr(clProgramBuildLog), size, slicePointer(buffer), 0)); err != nil {
			return "", err
		}
	}
	return strings.TrimRight(string(buffer), "\x00"), nil
}

func (e *openCLExecutor) Close() {
	if e == nil || e.api == nil {
		return
	}
	if e.result != 0 {
		clCode(e.api.releaseMemObject, e.result)
		e.result = 0
	}
	if e.blocks != 0 {
		clCode(e.api.releaseMemObject, e.blocks)
		e.blocks = 0
	}
	if e.kernel != 0 {
		clCode(e.api.releaseKernel, e.kernel)
		e.kernel = 0
	}
	if e.program != 0 {
		clCode(e.api.releaseProgram, e.program)
		e.program = 0
	}
	if e.queue != 0 {
		clCode(e.api.releaseCommandQueue, e.queue)
		e.queue = 0
	}
	if e.context != 0 {
		clCode(e.api.releaseContext, e.context)
		e.context = 0
	}
}

func (e *openCLExecutor) setMemoryArg(index uint32, value uintptr) error {
	return clCheck("clSetKernelArg", clCode(e.api.setKernelArg, e.kernel, uintptr(index), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&value))))
}

func (e *openCLExecutor) setUint32Arg(index uint32, value uint32) error {
	return clCheck("clSetKernelArg", clCode(e.api.setKernelArg, e.kernel, uintptr(index), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&value))))
}

func (e *openCLExecutor) writeBlocks(words []uint32) error {
	size := uintptr(len(words) * 4)
	if size > e.blockSize {
		return fmt.Errorf("OpenCL input is %d bytes, buffer is %d bytes", size, e.blockSize)
	}
	code := clCode(e.api.enqueueWriteBuffer, e.queue, e.blocks, clTrue, 0, size, slicePointer(words), 0, 0, 0)
	runtime.KeepAlive(words)
	return clCheck("clEnqueueWriteBuffer(input)", code)
}

func (e *openCLExecutor) run(templateCount, timestampStart, timestampCount, workCount uint32, allowed DigitSet, scope Scope, minRun, initialBest int) ([openCLResultWords]uint32, error) {
	if workCount == 0 || uint64(workCount) > maxGPUWorkItems {
		return [openCLResultWords]uint32{}, fmt.Errorf("OpenCL work count must be between 1 and %d", maxGPUWorkItems)
	}
	if templateCount == 0 || timestampCount == 0 || uint64(templateCount) > uint64(e.blockSize/64) || uint64(workCount) > uint64(templateCount)*uint64(timestampCount) {
		return [openCLResultWords]uint32{}, fmt.Errorf("invalid OpenCL dispatch dimensions: templates=%d timestamps=%d work=%d", templateCount, timestampCount, workCount)
	}
	result := [openCLResultWords]uint32{uint32(initialBest) << openCLResultIndexBits}
	// Only the winner/count header needs resetting. Hit slots are overwritten
	// by the kernel and read only up to the returned count.
	code := clCode(e.api.enqueueWriteBuffer, e.queue, e.result, clTrue, 0, 8, uintptr(unsafe.Pointer(&result[0])), 0, 0, 0)
	if err := clCheck("clEnqueueWriteBuffer(result)", code); err != nil {
		return result, err
	}
	scopeValue := uint32(0)
	if scope == ScopeAny {
		scopeValue = 1
	}
	args := []error{
		e.setUint32Arg(1, templateCount),
		e.setUint32Arg(2, timestampStart),
		e.setUint32Arg(3, timestampCount),
		e.setUint32Arg(4, workCount),
		e.setUint32Arg(5, uint32(allowed)),
		e.setUint32Arg(6, scopeValue),
		e.setUint32Arg(7, uint32(minRun)),
		e.setUint32Arg(9, uint32(initialBest)),
	}
	if err := errors.Join(args...); err != nil {
		return result, err
	}
	// The final budget claim can cover only a small part of the rectangle.
	// Keep the logical timestamp stride while avoiding entirely inactive rows.
	activeTemplates := (workCount-1)/timestampCount + 1
	activeTimestamps := min(timestampCount, workCount)
	globalSize := [2]uintptr{roundUpUintptr(uintptr(activeTimestamps), e.localSize), uintptr(activeTemplates)}
	localSize := [2]uintptr{e.localSize, 1}
	code = clCode(e.api.enqueueNDRangeKernel, e.queue, e.kernel, 2, 0, uintptr(unsafe.Pointer(&globalSize[0])), uintptr(unsafe.Pointer(&localSize[0])), 0, 0, 0)
	if err := clCheck("clEnqueueNDRangeKernel", code); err != nil {
		return result, err
	}
	code = clCode(e.api.enqueueReadBuffer, e.queue, e.result, clTrue, 0, 8, uintptr(unsafe.Pointer(&result[0])), 0, 0, 0)
	if err := clCheck("clEnqueueReadBuffer(result)", code); err != nil {
		return result, err
	}
	if hits := min(result[1], uint32(openCLHitCapacity)); hits > 0 {
		code = clCode(e.api.enqueueReadBuffer, e.queue, e.result, clTrue, 8, uintptr(hits)*4, uintptr(unsafe.Pointer(&result[2])), 0, 0, 0)
		if err := clCheck("clEnqueueReadBuffer(hits)", code); err != nil {
			return result, err
		}
	}
	// A blocking read on this in-order queue already waits for the kernel.
	return result, nil
}

func searchOpenCLWorker(ctx context.Context, cfg SearchConfig, device openCLDeviceRef, completed, reserved *atomic.Uint64, bestRun *atomic.Int32, output chan<- Candidate) error {
	if ctx.Err() != nil {
		return nil
	}
	keyBatch := cfg.GPUKeyBatch
	if keyBatch == 0 {
		keyBatch = defaultGPUKeyBatch
	}
	workItems := cfg.GPUWorkItems
	if workItems == 0 {
		workItems = defaultGPUWorkItems
	}
	workItems = retentionSafeGPUWorkItems(workItems, cfg.MinRun, cfg.Scope)
	workLimit := workItems
	if cfg.GPUWorkItems == 0 {
		// Start small on unknown hardware, then size each device independently
		// toward 100ms. Compilation time is excluded from the feedback loop.
		workItems = min(workItems, uint64(1<<20))
	}
	keyBatch = min(keyBatch, int(workItems))
	executor, err := newOpenCLExecutor(device, keyBatch)
	if err != nil {
		return err
	}
	defer executor.Close()

	timestampSpan := uint64(cfg.TimestampEnd) - uint64(cfg.TimestampStart) + 1
	keys := make([]*packetCandidate, 0, keyBatch)
	words := make([]uint32, 0, keyBatch*16)
	for {
		if ctx.Err() != nil {
			return nil
		}
		batchCount := keyBatch
		if cfg.MaxAttempts > 0 {
			used := reserved.Load()
			if used >= cfg.MaxAttempts {
				return nil
			}
			batchCount = int(min(uint64(batchCount), cfg.MaxAttempts-used))
		}
		keys, words = keys[:0], words[:0]
		for len(keys) < batchCount {
			if ctx.Err() != nil {
				return nil
			}
			privateKey, template, err := generateCandidateKey()
			if err != nil {
				return err
			}
			block, err := sha1PaddedBlock(template)
			if err != nil {
				return err
			}
			keys = append(keys, &packetCandidate{privateKey: privateKey, template: template})
			words = append(words, block[:]...)
		}
		if err := executor.writeBlocks(words); err != nil {
			return err
		}

		for cursor := uint64(0); cursor < timestampSpan; {
			if ctx.Err() != nil {
				return nil
			}
			timestampCount := workItems / uint64(len(keys))
			if timestampCount == 0 {
				timestampCount = 1
			}
			timestampCount = minUint64(timestampCount, timestampSpan-cursor)
			requested := uint64(len(keys)) * timestampCount
			claimed := claimAttempts(reserved, cfg.MaxAttempts, requested)
			if claimed == 0 {
				return nil
			}
			baseline := int(bestRun.Load())
			dispatchStart := cfg.TimestampStart + uint32(cursor)
			dispatchStarted := time.Now()
			gpuResult, err := executor.run(uint32(len(keys)), dispatchStart, uint32(timestampCount), uint32(claimed), cfg.AllowedDigits, cfg.Scope, cfg.MinRun, baseline)
			if err != nil {
				return err
			}
			if cfg.GPUWorkItems == 0 && claimed == requested {
				workItems = max(uint64(len(keys)), tuneGPUWorkItems(claimed, time.Since(dispatchStarted), workLimit))
			}
			completed.Add(claimed)
			cursor += timestampCount

			candidateFromScore := func(score uint32) (Candidate, error) {
				resultRun := int(score >> openCLResultIndexBits)
				resultIndex := uint64(score & uint32(maxGPUWorkItems))
				if resultIndex >= claimed || resultRun < 1 || resultRun > 16 {
					return Candidate{}, fmt.Errorf("OpenCL returned invalid score: index=%d work=%d run=%d", resultIndex, claimed, resultRun)
				}
				templateIndex := int(resultIndex / timestampCount)
				if templateIndex < 0 || templateIndex >= len(keys) {
					return Candidate{}, fmt.Errorf("OpenCL returned invalid template index %d", templateIndex)
				}
				timestamp := dispatchStart + uint32(resultIndex%timestampCount)
				fingerprint, keyID, err := fingerprintAt(keys[templateIndex].template, timestamp)
				if err != nil {
					return Candidate{}, err
				}
				match := EvaluateKeyIDForDigits(keyID, cfg.Scope, cfg.AllowedDigits)
				if match.RunLength != resultRun {
					return Candidate{}, fmt.Errorf("OpenCL verification mismatch: GPU run=%d, CPU key=%016X run=%d", resultRun, keyID, match.RunLength)
				}
				return Candidate{Fingerprint: fingerprint, KeyID: keyID, Timestamp: timestamp, Match: match, privateKey: keys[templateIndex].privateKey}, nil
			}

			hitCount := int(gpuResult[1])
			if hitCount > openCLHitCapacity {
				return fmt.Errorf("OpenCL qualified-hit buffer overflowed: %d hits in one dispatch (capacity %d); lower --gpu-work-items", hitCount, openCLHitCapacity)
			}
			bestScore := gpuResult[0]
			bestDelivered := false
			for hitIndex := 0; hitIndex < hitCount; hitIndex++ {
				score := gpuResult[2+hitIndex]
				candidate, err := candidateFromScore(score)
				if err != nil {
					return err
				}
				promoteBest(bestRun, candidate.Match.RunLength)
				output <- candidate
				bestDelivered = bestDelivered || score == bestScore
			}
			bestResultRun := int(bestScore >> openCLResultIndexBits)
			if !bestDelivered && bestResultRun > baseline {
				candidate, err := candidateFromScore(bestScore)
				if err != nil {
					return err
				}
				if promoteBest(bestRun, candidate.Match.RunLength) {
					output <- candidate
				}
			}
			if claimed < requested {
				return nil
			}
		}
	}
}

func tuneGPUWorkItems(previous uint64, elapsed time.Duration, limit uint64) uint64 {
	if elapsed <= 0 {
		return min(previous, limit)
	}
	const target = 100 * time.Millisecond
	if elapsed >= target*3/4 && elapsed <= target*5/4 {
		return min(previous, limit)
	}
	next := float64(previous) * float64(target) / float64(elapsed)
	// Limit growth to 2x per completed dispatch and cap before converting.
	next = min(next, float64(min(previous*2, limit)))
	return min(limit, max(uint64(min(4096, limit)), uint64(next)))
}

type packetCandidate struct {
	privateKey *packet.PrivateKey
	template   []byte
}

func highestPowerOfTwo(value uintptr) uintptr {
	if value == 0 {
		return 0
	}
	return uintptr(1) << (bits.Len64(uint64(value)) - 1)
}

func roundUpUintptr(value, multiple uintptr) uintptr {
	if multiple == 0 {
		return value
	}
	return (value + multiple - 1) / multiple * multiple
}

// Keep the expected number of qualifying results well below the fixed hit
// buffer for low thresholds. Run 12 is unaffected; this mainly makes small
// thresholds useful for tests and diagnostics without silently dropping hits.
func retentionSafeGPUWorkItems(configured uint64, minRun int, scope Scope) uint64 {
	safe := uint64(openCLHitCapacity / 4)
	exponent := minRun - 1
	if scope == ScopeAny {
		exponent--
	}
	for range max(0, exponent) {
		if safe > maxGPUWorkItems/16 {
			safe = maxGPUWorkItems
			break
		}
		safe *= 16
	}
	if safe == 0 {
		safe = 1
	}
	return minUint64(configured, safe)
}

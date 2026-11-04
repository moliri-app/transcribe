// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package ort binds the few calls of the ONNX Runtime C API (onnxruntime_c_api.h) that run a model
// with one float input and one float output, at runtime via purego (no cgo). The library is the
// onnxruntime of the speech recognition runtime (sherpa-onnx ships it next to its own library), so
// both share one runtime in the process, on the CPU or, in the CUDA build, on the graphics card.
package ort

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// apiVersion is the ORT_API_VERSION asked for: 1.17, older than every runtime the app downloads.
// The table of functions only grows, so the indices below stay valid in newer runtimes.
const apiVersion = 17

// Indices of the functions in struct OrtApi (onnxruntime_c_api.h of 1.17).
const (
	fnGetErrorMessage                = 2
	fnCreateEnv                      = 3
	fnCreateSession                  = 7
	fnRun                            = 9
	fnCreateSessionOptions           = 10
	fnDisableCpuMemArena             = 19
	fnSetSessionGraphOptimization    = 23
	fnSetIntraOpNumThreads           = 24
	fnSetInterOpNumThreads           = 25
	fnSessionGetInputName            = 36
	fnSessionGetOutputName           = 37
	fnCreateTensorWithDataAsOrtValue = 49
	fnGetTensorMutableData           = 51
	fnGetDimensionsCount             = 61
	fnGetDimensions                  = 62
	fnGetTensorTypeAndShape          = 65
	fnCreateCpuMemoryInfo            = 69
	fnAllocatorFree                  = 76
	fnGetAllocatorWithDefaultOptions = 78
	fnReleaseStatus                  = 93
	fnReleaseSession                 = 95
	fnReleaseValue                   = 96
	fnReleaseTensorTypeAndShapeInfo  = 99
	fnReleaseSessionOptions          = 100
	fnAppendCUDAV2                   = 204
	fnCreateCUDAProviderOptions      = 205
	fnUpdateCUDAProviderOptions      = 206
	fnReleaseCUDAProviderOptions     = 208
)

const (
	loggingError      = 3  // ORT_LOGGING_LEVEL_ERROR
	optimizeAll       = 99 // ORT_ENABLE_ALL
	arenaAllocator    = 1  // OrtArenaAllocator
	memTypeDefault    = 0  // OrtMemTypeDefault
	elementTypeFloat  = 1  // ONNX_TENSOR_ELEMENT_DATA_TYPE_FLOAT
	runtimeLogID      = "transcribe-ocr\x00"
	cudaDefaultDevice = "0"
)

// Runtime is the ONNX Runtime loaded into the process. There is one per process.
type Runtime struct {
	Dir     string // folder of the library
	Version string // e.g. "1.23.2"
	api     uintptr
	env     uintptr
	mem     uintptr
	alloc   uintptr
}

var (
	loadMu sync.Mutex
	loaded *Runtime
)

// Load loads the onnxruntime library of dir (once; later calls return the runtime loaded first,
// whatever dir they name, because a process can hold only one).
func Load(dir string) (*Runtime, error) {
	loadMu.Lock()
	defer loadMu.Unlock()
	if loaded != nil {
		return loaded, nil
	}
	h, err := openLib(dir)
	if err != nil {
		return nil, err
	}
	sym, err := symbol(h, "OrtGetApiBase")
	if err != nil {
		return nil, err
	}
	base, _, _ := purego.SyscallN(sym)
	if base == 0 {
		return nil, errors.New("onnxruntime: OrtGetApiBase returned nothing")
	}
	// struct OrtApiBase { const OrtApi* GetApi(uint32_t); const char* GetVersionString(void); }
	getAPI := *(*uintptr)(unsafe.Pointer(base))
	getVersion := *(*uintptr)(unsafe.Pointer(base + unsafe.Sizeof(uintptr(0))))
	v, _, _ := purego.SyscallN(getVersion)
	rt := &Runtime{Dir: dir, Version: goString(v)}
	rt.api, _, _ = purego.SyscallN(getAPI, apiVersion)
	if rt.api == 0 {
		return nil, fmt.Errorf("onnxruntime %s is older than 1.%d", rt.Version, apiVersion)
	}
	id := []byte(runtimeLogID)
	if err := rt.call(fnCreateEnv, loggingError, uintptr(unsafe.Pointer(&id[0])), uintptr(unsafe.Pointer(&rt.env))); err != nil {
		return nil, fmt.Errorf("onnxruntime: %w", err)
	}
	runtime.KeepAlive(id)
	if err := rt.call(fnCreateCpuMemoryInfo, arenaAllocator, memTypeDefault, uintptr(unsafe.Pointer(&rt.mem))); err != nil {
		return nil, fmt.Errorf("onnxruntime: %w", err)
	}
	if err := rt.call(fnGetAllocatorWithDefaultOptions, uintptr(unsafe.Pointer(&rt.alloc))); err != nil {
		return nil, fmt.Errorf("onnxruntime: %w", err)
	}
	loaded = rt
	return rt, nil
}

// LibPresent reports whether dir holds an onnxruntime library.
func LibPresent(dir string) bool {
	_, err := libPath(dir)
	return err == nil
}

func libPath(dir string) (string, error) {
	var pats []string
	switch runtime.GOOS {
	case "windows":
		pats = []string{"onnxruntime.dll"}
	case "darwin":
		pats = []string{"libonnxruntime.dylib", "libonnxruntime.*.dylib"}
	default:
		pats = []string{"libonnxruntime.so", "libonnxruntime.so.*"}
	}
	for _, p := range pats {
		m, _ := filepath.Glob(filepath.Join(dir, p))
		for _, f := range m {
			if st, err := os.Stat(f); err == nil && !st.IsDir() {
				return f, nil
			}
		}
	}
	return "", fmt.Errorf("no onnxruntime library in %s", dir)
}

func (rt *Runtime) fn(i int) uintptr {
	return *(*uintptr)(unsafe.Pointer(rt.api + uintptr(i)*unsafe.Sizeof(uintptr(0))))
}

// call calls a function of the table that returns an OrtStatus* and turns that into an error.
func (rt *Runtime) call(i int, args ...uintptr) error {
	st, _, _ := purego.SyscallN(rt.fn(i), args...)
	if st == 0 {
		return nil
	}
	msg, _, _ := purego.SyscallN(rt.fn(fnGetErrorMessage), st)
	err := errors.New(goString(msg))
	purego.SyscallN(rt.fn(fnReleaseStatus), st)
	return err
}

// Session is one model loaded for running.
type Session struct {
	rt      *Runtime
	h       uintptr
	in, out []byte // the names of the first input and output, NUL-terminated
	GPU     bool   // runs on the graphics card
	mu      sync.Mutex
}

// NewSession loads the model at path. threads bounds the cores one run uses (0: the runtime's
// default); cuda puts it on the graphics card when the runtime is the CUDA build, else it runs on
// the CPU.
func (rt *Runtime) NewSession(path string, threads int, cuda bool) (*Session, error) {
	var so uintptr
	if err := rt.call(fnCreateSessionOptions, uintptr(unsafe.Pointer(&so))); err != nil {
		return nil, err
	}
	defer purego.SyscallN(rt.fn(fnReleaseSessionOptions), so)
	if threads > 0 {
		if err := rt.call(fnSetIntraOpNumThreads, so, uintptr(threads)); err != nil {
			return nil, err
		}
	}
	if err := rt.call(fnSetInterOpNumThreads, so, 1); err != nil {
		return nil, err
	}
	// images of every size: an arena would keep the largest one's memory
	if err := rt.call(fnDisableCpuMemArena, so); err != nil {
		return nil, err
	}
	if err := rt.call(fnSetSessionGraphOptimization, so, optimizeAll); err != nil {
		return nil, err
	}
	gpu := cuda && rt.appendCUDA(so) == nil

	p, err := nativePath(path)
	if err != nil {
		return nil, err
	}
	s := &Session{rt: rt, GPU: gpu}
	err = rt.call(fnCreateSession, rt.env, uintptr(p), so, uintptr(unsafe.Pointer(&s.h)))
	runtime.KeepAlive(p)
	if err != nil {
		return nil, fmt.Errorf("onnxruntime: %s: %w", filepath.Base(path), err)
	}
	if s.in, err = s.name(fnSessionGetInputName); err == nil {
		s.out, err = s.name(fnSessionGetOutputName)
	}
	if err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// appendCUDA asks for the CUDA execution provider; the CPU build of the runtime refuses it.
// Convolution algorithms are chosen by heuristic, not by trying them all: detection sees a new
// image size with every page, and an exhaustive search would run again for each.
func (rt *Runtime) appendCUDA(so uintptr) error {
	var opts uintptr
	if err := rt.call(fnCreateCUDAProviderOptions, uintptr(unsafe.Pointer(&opts))); err != nil {
		return err
	}
	defer purego.SyscallN(rt.fn(fnReleaseCUDAProviderOptions), opts)
	k := newCStrings("device_id", "cudnn_conv_algo_search", "arena_extend_strategy")
	v := newCStrings(cudaDefaultDevice, "HEURISTIC", "kSameAsRequested")
	err := rt.call(fnUpdateCUDAProviderOptions, opts, k.ptr(), v.ptr(), uintptr(len(k.p)))
	runtime.KeepAlive(k)
	runtime.KeepAlive(v)
	if err != nil {
		return err
	}
	return rt.call(fnAppendCUDAV2, so, opts)
}

func (s *Session) name(fn int) ([]byte, error) {
	var p uintptr
	if err := s.rt.call(fn, s.h, 0, s.rt.alloc, uintptr(unsafe.Pointer(&p))); err != nil {
		return nil, err
	}
	name := append([]byte(goString(p)), 0)
	_ = s.rt.call(fnAllocatorFree, s.rt.alloc, p)
	return name, nil
}

// Run runs the model on one float tensor of the given shape and returns its first output and the
// output's shape.
func (s *Session) Run(input []float32, shape []int64) ([]float32, []int64, error) {
	n := int64(1)
	for _, d := range shape {
		n *= d
	}
	if n != int64(len(input)) || n == 0 {
		return nil, nil, fmt.Errorf("onnxruntime: %d values for shape %v", len(input), shape)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.h == 0 {
		return nil, nil, errors.New("onnxruntime: session closed")
	}
	rt := s.rt
	var pin runtime.Pinner
	defer pin.Unpin()
	pin.Pin(&input[0])
	pin.Pin(&shape[0])
	pin.Pin(&s.in[0])
	pin.Pin(&s.out[0])

	var in uintptr
	if err := rt.call(fnCreateTensorWithDataAsOrtValue, rt.mem, uintptr(unsafe.Pointer(&input[0])), uintptr(len(input)*4),
		uintptr(unsafe.Pointer(&shape[0])), uintptr(len(shape)), elementTypeFloat, uintptr(unsafe.Pointer(&in))); err != nil {
		return nil, nil, err
	}
	defer purego.SyscallN(rt.fn(fnReleaseValue), in)

	names := []uintptr{uintptr(unsafe.Pointer(&s.in[0])), uintptr(unsafe.Pointer(&s.out[0]))}
	values := []uintptr{in, 0}
	pin.Pin(&names[0])
	pin.Pin(&values[0])
	if err := rt.call(fnRun, s.h, 0, uintptr(unsafe.Pointer(&names[0])), uintptr(unsafe.Pointer(&values[0])), 1,
		uintptr(unsafe.Pointer(&names[1])), 1, uintptr(unsafe.Pointer(&values[1]))); err != nil {
		return nil, nil, err
	}
	out := values[1]
	defer purego.SyscallN(rt.fn(fnReleaseValue), out)

	var info, dims uintptr
	if err := rt.call(fnGetTensorTypeAndShape, out, uintptr(unsafe.Pointer(&info))); err != nil {
		return nil, nil, err
	}
	defer purego.SyscallN(rt.fn(fnReleaseTensorTypeAndShapeInfo), info)
	if err := rt.call(fnGetDimensionsCount, info, uintptr(unsafe.Pointer(&dims))); err != nil {
		return nil, nil, err
	}
	outShape := make([]int64, dims)
	if dims > 0 {
		pin.Pin(&outShape[0])
		if err := rt.call(fnGetDimensions, info, uintptr(unsafe.Pointer(&outShape[0])), dims); err != nil {
			return nil, nil, err
		}
	}
	m := int64(1)
	for _, d := range outShape {
		m *= d
	}
	var data uintptr
	if err := rt.call(fnGetTensorMutableData, out, uintptr(unsafe.Pointer(&data))); err != nil {
		return nil, nil, err
	}
	res := make([]float32, m)
	if m > 0 {
		copy(res, unsafe.Slice((*float32)(unsafe.Pointer(data)), m))
	}
	return res, outShape, nil
}

// Close frees the session.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.h != 0 {
		purego.SyscallN(s.rt.fn(fnReleaseSession), s.h)
		s.h = 0
	}
}

// cStrings keeps NUL-terminated strings and an array of pointers to them alive for one call.
type cStrings struct {
	b [][]byte
	p []uintptr
}

func newCStrings(ss ...string) *cStrings {
	c := &cStrings{}
	for _, s := range ss {
		b := append([]byte(s), 0)
		c.b = append(c.b, b)
		c.p = append(c.p, uintptr(unsafe.Pointer(&b[0])))
	}
	return c
}

func (c *cStrings) ptr() uintptr { return uintptr(unsafe.Pointer(&c.p[0])) }

func goString(p uintptr) string {
	if p == 0 {
		return ""
	}
	var n int
	for *(*byte)(unsafe.Pointer(p + uintptr(n))) != 0 {
		n++
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(p)), n))
}

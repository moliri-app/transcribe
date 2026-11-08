// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package setup

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/moliri-app/transcribe/internal/llm"
	"github.com/moliri-app/transcribe/models"
	"github.com/moliri-app/transcribe/proc"
)

// The language model runs on the graphics card where there is one it is faster on: llama.cpp's
// CUDA build where the computer has an NVIDIA graphics card (the one speech recognition uses),
// otherwise its Vulkan build (any vendor, also the graphics built into Intel and AMD
// processors) where it measured faster than the processor here, otherwise the processor. The
// choice is made once, when the model has come or on its first start after an update, and kept
// in <data>/llm/device.json. A graphics card that fails leaves the model to the CPU build
// (internal/llm), which stays in place for that.

// llamaGPUArchives are the builds for graphics cards, Windows on x64 only; CUDA's comes with
// NVIDIA's runtime libraries, which llama.cpp publishes beside it.
var llamaGPUArchives = map[string][]llamaArchive{
	"cuda13": {
		{"llama-b12110-bin-win-cuda-13.4-x64.zip", "dc910a969190b5d42e189afd73117f6bbdd8dd8ed6586342c5e0b82cab62c673"},
		{"cudart-llama-bin-win-cuda-13.4-x64.zip", "738f8c251ac22b70c3ae6f83a10cf222725df0395246a2cf58f32bdb85fbe668"},
	},
	"cuda12": {
		{"llama-b12110-bin-win-cuda-12.4-x64.zip", "1ce755bc3a061889234048f88e33328e17971eaad73c4666cfeb26026b317fee"},
		{"cudart-llama-bin-win-cuda-12.4-x64.zip", "8c79a9b226de4b3cacfd1f83d24f962d0773be79f1e7b75c6af4ded7e32ae1d6"},
	},
	"vulkan": {
		{"llama-b12110-bin-win-vulkan-x64.zip", "9732a5e48887ca54d0bfe4af4324dc85fd56cd8cda19b52df3542fa6ebe6874f"},
	},
}

// gpuArgs put as many layers on the graphics card as fit, a quarter of a GB to spare, the rest
// stays on the processor (llama.cpp's --fit). Every layer counts: on a laptop's RTX A3000 with
// 6 GB (5 GB free) the model wrote 7 tokens a second with 21 of its 29 layers on the card (the
// default margin of 1 GB), 47 with all. For CUDA the cache of what the model read takes half the
// memory (8 bits) and the batches half the room, which puts all of them there.
var (
	cudaArgs   = []string{"-ngl", "auto", "-fitt", "256", "-ub", "256", "-ctk", "q8_0", "-ctv", "q8_0"}
	vulkanArgs = []string{"-ngl", "auto", "-fitt", "256"}
)

func gpuArgs(build string) []string {
	if build == "vulkan" {
		return vulkanArgs
	}
	return cudaArgs
}

// llmChoice is the build picked for this computer.
type llmChoice struct {
	// Build: "cuda13", "cuda12", "vulkan" or "cpu".
	Build string `json:"build"`
	// Device is the graphics card, for people.
	Device string `json:"device,omitempty"`
	// Speeds measured, by build (Vulkan against the processor).
	Speeds map[string]llm.Speed `json:"speeds,omitempty"`
}

var llmChoiceMu sync.Mutex

// detectGPU finds an NVIDIA graphics card (a test passes over it to try Vulkan).
var detectGPU = DetectGPU

func llmChoiceFile(dataDir string) string { return filepath.Join(LLMDir(dataDir), "device.json") }

// llamaBuildDir holds a build: the CPU build in LlamaDir, the others beside it (llama-cuda13).
func llamaBuildDir(dataDir, build string) string {
	if build == "cpu" {
		return LlamaDir(dataDir)
	}
	return filepath.Join(LLMDir(dataDir), "llama-"+build)
}

func llamaBuildServer(dataDir, build string) string {
	return filepath.Join(llamaBuildDir(dataDir, build), filepath.Base(LlamaServer(dataDir)))
}

func cpuBuild(dataDir string) llm.Build {
	return llm.Build{Name: "cpu", Program: LlamaServer(dataDir), Args: []string{"-ngl", "0"}}
}

// LLMBuilds returns llama.cpp's builds for the model, best first and the CPU build last, picking
// the build for this computer first if that is not done yet (which may download it and measure
// it, a minute or more). Only once the model is downloaded.
func LLMBuilds(ctx context.Context, dataDir string) ([]llm.Build, error) {
	cpu := cpuBuild(dataDir)
	c := chooseLLMBuild(ctx, dataDir)
	if c.Build == "cpu" {
		return []llm.Build{cpu}, nil
	}
	gpu := llm.Build{Name: c.Build, Program: llamaBuildServer(dataDir, c.Build), Args: gpuArgs(c.Build)}
	return []llm.Build{gpu, cpu}, nil
}

// LLMDevice names the device the model runs on when it is loaded: the graphics card, or "" for
// the processor and while nothing is picked yet.
func LLMDevice(dataDir string) (build, device string) {
	var c llmChoice
	data, err := os.ReadFile(llmChoiceFile(dataDir))
	if err != nil || json.Unmarshal(data, &c) != nil {
		return "", ""
	}
	return c.Build, c.Device
}

// chooseLLMBuild returns the build picked for this computer, picking it first if need be. When
// that fails (no internet for the GPU build), the processor answers and the next start tries
// again.
func chooseLLMBuild(ctx context.Context, dataDir string) llmChoice {
	llmChoiceMu.Lock()
	defer llmChoiceMu.Unlock()
	var c llmChoice
	if data, err := os.ReadFile(llmChoiceFile(dataDir)); err == nil && json.Unmarshal(data, &c) == nil && c.Build != "" {
		if _, err := os.Stat(llamaBuildServer(dataDir, c.Build)); err == nil || c.Build == "cpu" {
			return c
		}
		// The GPU build went missing: fetch it again.
		if archives, ok := llamaGPUArchives[c.Build]; ok {
			err := fetchLlamaInto(ctx, dataDir, llamaBuildDir(dataDir, c.Build), archives)
			models.Track("llm", trackState(err), 0, 0, err)
			if err == nil {
				return c
			}
			log.Println("llm: fetching", c.Build+":", err)
		}
		return llmChoice{Build: "cpu"}
	}
	c, err := pickLLMBuild(ctx, dataDir)
	if err != nil {
		log.Println("llm: picking the device:", err)
		return llmChoice{Build: "cpu"}
	}
	if data, err := json.MarshalIndent(c, "", "  "); err == nil {
		os.WriteFile(llmChoiceFile(dataDir), data, 0o644)
	}
	log.Printf("llm: runs on %s %s", c.Build, c.Device)
	return c
}

func trackState(err error) string {
	if err != nil {
		return "error"
	}
	return "done"
}

// pickLLMBuild finds the build for this computer and fetches it.
func pickLLMBuild(ctx context.Context, dataDir string) (llmChoice, error) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return llmChoice{Build: "cpu"}, nil
	}
	if g := detectGPU(""); g.Present && g.CudaDriver >= 12 {
		name := "cuda12"
		if g.CudaDriver >= 13 {
			name = "cuda13"
		}
		err := fetchLlamaInto(ctx, dataDir, llamaBuildDir(dataDir, name), llamaGPUArchives[name])
		models.Track("llm", trackState(err), 0, 0, err)
		if err != nil {
			return llmChoice{}, err
		}
		return llmChoice{Build: name, Device: g.Name}, nil
	}

	// Any other graphics card: Vulkan, if it is faster than the processor here.
	dir := llamaBuildDir(dataDir, "vulkan")
	err := fetchLlamaInto(ctx, dataDir, dir, llamaGPUArchives["vulkan"])
	models.Track("llm", trackState(err), 0, 0, err)
	if err != nil {
		return llmChoice{}, err
	}
	vulkan := llm.Build{Name: "vulkan", Program: llamaBuildServer(dataDir, "vulkan"), Args: vulkanArgs}
	device := vulkanDevice(ctx, vulkan.Program)
	if device == "" {
		os.RemoveAll(dir)
		return llmChoice{Build: "cpu"}, nil
	}
	c := llmChoice{Build: "cpu", Speeds: map[string]llm.Speed{}}
	cpu, err := measure(ctx, cpuBuild(dataDir), LLMModel(dataDir))
	if err != nil {
		return llmChoice{}, err
	}
	c.Speeds["cpu"] = cpu
	gpu, err := measure(ctx, vulkan, LLMModel(dataDir))
	if err != nil {
		if ctx.Err() != nil {
			return llmChoice{}, err
		}
		log.Println("llm: Vulkan on", device+":", err)
	}
	c.Speeds["vulkan"] = gpu
	// Measurably faster: a tenth less time for a question at least.
	if err == nil && gpu.Seconds() > 0 && gpu.Seconds() < 0.9*cpu.Seconds() {
		c.Build, c.Device = "vulkan", device
	} else {
		os.RemoveAll(dir)
	}
	return c, nil
}

func measure(ctx context.Context, b llm.Build, model string) (llm.Speed, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	sp, err := llm.Measure(ctx, b, model)
	log.Printf("llm: %s reads %.1f and writes %.1f tokens a second (%v)", b.Name, sp.Read, sp.Write, err)
	return sp, err
}

// vulkanDevice names the graphics cards the Vulkan build finds ("" for none): llama.cpp spreads
// the model over all of them.
var vulkanLine = regexp.MustCompile(`(?m)^\s*Vulkan\d+:\s*(.+?)\s*\(\d+ MiB`)

func vulkanDevice(ctx context.Context, program string) string {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := proc.Hide(exec.CommandContext(ctx, program, "--list-devices"))
	cmd.Dir = filepath.Dir(program)
	out, _ := cmd.CombinedOutput()
	var names []string
	for _, m := range vulkanLine.FindAllStringSubmatch(string(out), -1) {
		names = append(names, strings.TrimSpace(m[1]))
	}
	return strings.Join(names, " + ")
}

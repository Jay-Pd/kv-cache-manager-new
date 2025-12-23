/*
Copyright 2025 The llm-d Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package preprocessing

//nolint: gocritic // C and unsafe are considered dups by the linter.
import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"unsafe"

	/*
		#include "cgo_functions.h"
	*/
	"C"

	"github.com/llm-d/llm-d-kv-cache-manager/pkg/utils/logging"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RenderJinjaTemplateRequest represents the request to render a chat template.
type RenderJinjaTemplateRequest struct {
	Conversations             []ChatMessage          `json:"messages"`
	Tools                     []interface{}          `json:"tools,omitempty"`
	Documents                 []interface{}          `json:"documents,omitempty"`
	ChatTemplate              string                 `json:"chat_template,omitempty"`
	ReturnAssistantTokensMask bool                   `json:"return_assistant_tokens_mask,omitempty"`
	ContinueFinalMessage      bool                   `json:"continue_final_message,omitempty"`
	AddGenerationPrompt       bool                   `json:"add_generation_prompt,omitempty"`
	ChatTemplateKWArgs        map[string]interface{} `json:"chat_template_kwargs,omitempty"`
}

// DeepCopy creates a deep copy of the RenderJinjaTemplateRequest.
func (req *RenderJinjaTemplateRequest) DeepCopy() (*RenderJinjaTemplateRequest, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var out RenderJinjaTemplateRequest
	err = json.Unmarshal(b, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// RenderJinjaTemplateResponse represents the response from rendering a chat template.
type RenderJinjaTemplateResponse struct {
	RenderedChats     []string  `json:"rendered_chats"`
	GenerationIndices [][][]int `json:"generation_indices"`
}

// FetchChatTemplateRequest represents the request to fetch a chat template.
type FetchChatTemplateRequest struct {
	Model        string        `json:"model"`
	ChatTemplate string        `json:"chat_template,omitempty"`
	Tools        []interface{} `json:"tools,omitempty"`
	Revision     string        `json:"revision,omitempty"`
	Token        string        `json:"token,omitempty"`
	IsLocalPath  bool          `json:"is_local_path,omitempty"`
}

// FetchChatTemplateResponse represents the response from fetching a chat template.
type FetchChatTemplateResponse struct {
	ChatTemplate       string                 `json:"chat_template,omitempty"`
	ChatTemplateKWArgs map[string]interface{} `json:"chat_template_kwargs,omitempty"`
}

// ChatTemplatingProcessor is a processor that handles chat template rendering
type ChatTemplatingProcessor struct{}

// NewChatTemplatingProcessor creates a new instance of ChatTemplatingProcessor.
func NewChatTemplatingProcessor() *ChatTemplatingProcessor {
	return &ChatTemplatingProcessor{}
}

// logMem logs the current memory usage
func logMem(ctx context.Context, label string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	traceLogger := log.FromContext(ctx).V(logging.TRACE).WithName("MemoryStats")
	traceLogger.Info(label,
		"Alloc", m.Alloc,
		"TotalAlloc", m.TotalAlloc,
		"Sys", m.Sys,
		"NumGC", m.NumGC,
	)
}

// Initialize initializes the Python interpreter and caches the module.
func (w *ChatTemplatingProcessor) Initialize() error {
	fmt.Println("Initialize called")
	logMem(context.Background(), "Before Initialize")

	C.Py_InitializeGo()

	result := C.Py_InitChatTemplateModule()
	if result != 0 {
		return fmt.Errorf("failed to initialize chat template module")
	}

	logMem(context.Background(), "After Initialize")
	return nil
}

// Finalize finalizes the Python interpreter and cleans up the module.
func (w *ChatTemplatingProcessor) Finalize() {
	fmt.Println("Finalize called")
	logMem(context.Background(), "Before Finalize")

	C.Py_CleanupChatTemplateModule()
	C.Py_FinalizeGo()

	logMem(context.Background(), "After Finalize")
}

// RenderChatTemplate renders a chat template using the cached Python function.
func (w *ChatTemplatingProcessor) RenderChatTemplate(ctx context.Context,
	req *RenderJinjaTemplateRequest,
) (*RenderJinjaTemplateResponse, error) {
	fmt.Println("RenderChatTemplate called")
	logMem(ctx, "Before RenderChatTemplate")

	if req == nil {
		return nil, fmt.Errorf("received nil request")
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	cReqJSON := C.CString(string(reqJSON))
	defer C.free(unsafe.Pointer(cReqJSON))

	cResult := C.Py_CallRenderJinjaTemplate(cReqJSON)
	if cResult == nil {
		return nil, fmt.Errorf("python render_jinja_template failed")
	}
	defer C.free(unsafe.Pointer(cResult))

	resultJSON := C.GoString(cResult)

	var response RenderJinjaTemplateResponse
	if err := json.Unmarshal([]byte(resultJSON), &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	logMem(ctx, "After RenderChatTemplate")
	return &response, nil
}

// FetchChatTemplate fetches the model chat template using the cached Python function.
func (w *ChatTemplatingProcessor) FetchChatTemplate(
	ctx context.Context,
	req FetchChatTemplateRequest,
) (string, map[string]interface{}, error) {
	fmt.Println("FetchChatTemplate called")
	logMem(ctx, "Before FetchChatTemplate")

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	cReqJSON := C.CString(string(reqJSON))
	defer C.free(unsafe.Pointer(cReqJSON))

	cResult := C.Py_CallGetModelChatTemplate(cReqJSON)
	if cResult == nil {
		return "", nil, fmt.Errorf("python get_model_chat_template failed")
	}
	defer C.free(unsafe.Pointer(cResult))

	resultJSON := C.GoString(cResult)

	var response FetchChatTemplateResponse
	if err := json.Unmarshal([]byte(resultJSON), &response); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	logMem(ctx, "After FetchChatTemplate")
	return response.ChatTemplate, response.ChatTemplateKWArgs, nil
}

// ClearCaches clears all caches for testing purposes.
func ClearCaches(ctx context.Context) error {
	fmt.Println("ClearCaches called")
	logMem(ctx, "Before ClearCaches")

	cResult := C.Py_ClearCaches()
	if cResult == nil {
		return fmt.Errorf("failed to clear caches")
	}
	defer C.free(unsafe.Pointer(cResult))

	logMem(ctx, "After ClearCaches")
	return nil
}

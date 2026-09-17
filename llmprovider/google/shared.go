package google

import (
	"github.com/mudler/cogito"
	openai "github.com/sashabaranov/go-openai"
)

// TranslateRequest translates an OpenAI Chat Completion request to a Gemini
// generateContent JSON body. Exported for reuse by other Gemini-based adapters
// (e.g., google-vertex).
func TranslateRequest(req openai.ChatCompletionRequest) ([]byte, error) {
	var l LLM
	return l.translateRequest(req)
}

// TranslateResponse parses a Gemini generateContent JSON response into OpenAI
// types. Exported for reuse by other Gemini-based adapters.
func TranslateResponse(body []byte, requestModel string) (cogito.LLMReply, cogito.LLMUsage, error) {
	var l LLM
	return l.translateResponse(body, requestModel)
}

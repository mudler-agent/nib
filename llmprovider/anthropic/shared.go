package anthropic

import (
	"github.com/mudler/cogito"
	openai "github.com/sashabaranov/go-openai"
)

// TranslateRequest converts an OpenAI Chat Completion request to an Anthropic
// Messages API JSON body. Exported for reuse by the GitHub Copilot adapter
// and other multi-protocol providers that need Anthropic Messages routing.
func TranslateRequest(req openai.ChatCompletionRequest) ([]byte, error) {
	var l LLM
	return l.translateRequest(req)
}

// TranslateResponse parses an Anthropic Messages API JSON response into OpenAI
// types. Exported for reuse by multi-protocol adapters.
func TranslateResponse(body []byte, requestModel string) (cogito.LLMReply, cogito.LLMUsage, error) {
	var l LLM
	return l.translateResponse(body, requestModel)
}

// ParseAPIError formats an HTTP error response body into a Go error.
// Exported for reuse by multi-protocol adapters.
func ParseAPIError(status int, body []byte) error {
	return parseAPIError(status, body)
}

// ErrNoResponseSentinel is returned when the API completes without an
// assistant message. Exported for reuse.
var ErrNoResponseSentinel = ErrNoResponse

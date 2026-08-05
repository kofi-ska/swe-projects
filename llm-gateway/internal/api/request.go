package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

type GenerateRequest struct {
	Prompt    string `json:"prompt"`
	MaxTokens int    `json:"max_tokens"`
	Priority  string `json:"priority"`
}

func decodeGenerateRequest(writer http.ResponseWriter, request *http.Request) (GenerateRequest, error) {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var input GenerateRequest
	if err := decoder.Decode(&input); err != nil {
		return GenerateRequest{}, fmt.Errorf("invalid JSON request: %w", err)
	}
	if err := ensureEOF(decoder); err != nil {
		return GenerateRequest{}, errors.New("request body must contain one JSON object")
	}
	if err := validate(input); err != nil {
		return GenerateRequest{}, err
	}
	return input, nil
}

func validate(request GenerateRequest) error {
	if strings.TrimSpace(request.Prompt) == "" {
		return errors.New("prompt must not be empty")
	}
	if utf8.RuneCountInString(request.Prompt) > 8000 {
		return errors.New("prompt must not exceed 8000 characters")
	}
	if request.MaxTokens < 1 || request.MaxTokens > 500 {
		return errors.New("max_tokens must be between 1 and 500")
	}
	switch request.Priority {
	case "low", "normal", "high":
		return nil
	default:
		return errors.New("priority must be low, normal, or high")
	}
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	return errors.New("unexpected trailing JSON value")
}

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
)

func (s *modelService) ollamaBaseURL() string {
	if value := strings.TrimSpace(os.Getenv("OLLAMA_BASE_URL")); value != "" {
		return strings.TrimRight(value, "/")
	}
	return "http://localhost:11434/api"
}

func (s *modelService) doJSONRequest(method, targetURL string, payload interface{}, headers http.Header) (int, map[string]interface{}, error) {
	var bodyReader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		bodyReader = bytes.NewReader(raw)
	}

	request, err := http.NewRequestWithContext(context.Background(), method, targetURL, bodyReader)
	if err != nil {
		return 0, nil, err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}

	response, err := s.httpClient.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()

	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, nil, err
	}
	if len(rawBody) == 0 {
		return response.StatusCode, map[string]interface{}{}, nil
	}

	result := map[string]interface{}{}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		result["raw"] = string(rawBody)
	}
	return response.StatusCode, result, nil
}

func (s *modelService) doMultipartTranscription(targetURL, modelName string, fileData []byte, fileName string, headers http.Header) (int, map[string]interface{}, error) {
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)

	_ = writer.WriteField("model", modelName)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return 0, nil, err
	}
	if _, err := part.Write(fileData); err != nil {
		return 0, nil, err
	}
	if err := writer.Close(); err != nil {
		return 0, nil, err
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, targetURL, buffer)
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}

	response, err := s.httpClient.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()

	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, nil, err
	}
	result := map[string]interface{}{}
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &result); err != nil {
			result["raw"] = string(rawBody)
		}
	}
	return response.StatusCode, result, nil
}

func buildAuthHeaders(apiKey string, customHeaders map[string]string) http.Header {
	headers := http.Header{}
	if strings.TrimSpace(apiKey) != "" {
		headers.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}
	for key, value := range sanitizeHeaders(customHeaders) {
		headers.Set(key, value)
	}
	return headers
}

func sanitizeHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	result := map[string]string{}
	for key, value := range headers {
		trimmedKey := textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(key))
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		if strings.EqualFold(trimmedKey, "Authorization") || strings.EqualFold(trimmedKey, "Content-Type") {
			continue
		}
		result[trimmedKey] = trimmedValue
	}
	return result
}

func appendPath(baseURL, suffix string) string {
	if strings.TrimSpace(baseURL) == "" {
		return ""
	}
	baseURL = strings.TrimRight(baseURL, "/")
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return baseURL
	}
	if strings.HasPrefix(strings.ToLower(baseURL), "http://") || strings.HasPrefix(strings.ToLower(baseURL), "https://") {
		if strings.HasSuffix(baseURL, "/"+strings.TrimLeft(suffix, "/")) {
			return baseURL
		}
	}
	if strings.Contains(strings.ToLower(baseURL), "/"+strings.ToLower(strings.TrimLeft(suffix, "/"))) {
		return baseURL
	}
	return baseURL + "/" + strings.TrimLeft(suffix, "/")
}

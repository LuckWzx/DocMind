package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"
	"time"

	dto "docmind/internal/model/dto/response"
	pkgerrors "docmind/pkg/errors"

	"github.com/google/uuid"
)

// ollamaDownloadTask 记录单次 Ollama 模型下载任务的状态与进度。
type ollamaDownloadTask struct {
	ID        string
	ModelName string
	Status    string
	Progress  float64
	Message   string
	StartTime time.Time
	EndTime   *time.Time
}

// ---------- Ollama 状态与模型列表 ----------

// GetOllamaStatus 探测本地 Ollama 服务是否可达并获取版本号。
func (s *modelService) GetOllamaStatus() (*dto.OllamaStatusResponse, error) {
	url := appendPath(s.ollamaBaseURL(), "version")
	status, body, err := s.doJSONRequest(http.MethodGet, url, nil, nil)
	if err != nil {
		return &dto.OllamaStatusResponse{
			Available: false,
			Error:     err.Error(),
			BaseURL:   s.ollamaBaseURL(),
		}, nil
	}
	if status >= 200 && status < 300 {
		version, _ := body["version"].(string)
		return &dto.OllamaStatusResponse{
			Available: true,
			Version:   version,
			BaseURL:   s.ollamaBaseURL(),
		}, nil
	}
	return &dto.OllamaStatusResponse{
		Available: false,
		Error:     extractErrorMessage(body, status),
		BaseURL:   s.ollamaBaseURL(),
	}, nil
}

// ListOllamaModels 获取已安装的 Ollama 模型列表。
func (s *modelService) ListOllamaModels() ([]*dto.OllamaModelInfoResponse, error) {
	status, body, err := s.doJSONRequest(http.MethodGet, appendPath(s.ollamaBaseURL(), "tags"), nil, nil)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeServiceUnavailable, "获取 Ollama 模型列表失败", err)
	}
	if status < 200 || status >= 300 {
		return nil, pkgerrors.New(pkgerrors.CodeServiceUnavailable, extractErrorMessage(body, status))
	}

	modelsRaw, _ := body["models"].([]interface{})
	result := make([]*dto.OllamaModelInfoResponse, 0, len(modelsRaw))
	for _, item := range modelsRaw {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		result = append(result, &dto.OllamaModelInfoResponse{
			Name:       stringValue(itemMap["name"]),
			Size:       int64Value(itemMap["size"]),
			Digest:     stringValue(itemMap["digest"]),
			ModifiedAt: stringValue(itemMap["modified_at"]),
		})
	}
	return result, nil
}

// CheckOllamaModels 批量检查指定模型是否在 Ollama 中已安装。
func (s *modelService) CheckOllamaModels(models []string) (map[string]bool, error) {
	list, err := s.ListOllamaModels()
	if err != nil {
		return nil, err
	}

	installed := map[string]bool{}
	for _, model := range list {
		installed[model.Name] = true
	}

	result := map[string]bool{}
	for _, name := range models {
		result[name] = installed[name]
	}
	return result, nil
}

// ---------- Ollama 模型下载 ----------

// DownloadOllamaModel 异步下载 Ollama 模型，返回任务 ID 供前端轮询进度。
func (s *modelService) DownloadOllamaModel(modelName string) (*dto.DownloadTaskResponse, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidParam, "模型名称不能为空")
	}

	task := &ollamaDownloadTask{
		ID:        uuid.NewString(),
		ModelName: modelName,
		Status:    "pending",
		Progress:  0,
		Message:   "下载任务已创建",
		StartTime: time.Now(),
	}

	s.taskMu.Lock()
	s.tasks[task.ID] = task
	s.taskMu.Unlock()

	go s.runOllamaDownload(task)
	return s.toDownloadTaskResponse(task), nil
}

// GetOllamaDownloadProgress 按任务 ID 查询下载进度。
func (s *modelService) GetOllamaDownloadProgress(taskID string) (*dto.DownloadTaskResponse, error) {
	s.taskMu.RLock()
	task, ok := s.tasks[taskID]
	s.taskMu.RUnlock()
	if !ok {
		return nil, pkgerrors.New(pkgerrors.CodeResourceNotFound, "下载任务不存在")
	}
	return s.toDownloadTaskResponse(task), nil
}

// ListOllamaDownloadTasks 列出所有下载任务（按创建时间倒序）。
func (s *modelService) ListOllamaDownloadTasks() ([]*dto.DownloadTaskResponse, error) {
	s.taskMu.RLock()
	tasks := make([]*ollamaDownloadTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	s.taskMu.RUnlock()

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].StartTime.After(tasks[j].StartTime)
	})

	result := make([]*dto.DownloadTaskResponse, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, s.toDownloadTaskResponse(task))
	}
	return result, nil
}

// runOllamaDownload 在后台 goroutine 中执行 Ollama pull，解析 SSE 进度事件。
func (s *modelService) runOllamaDownload(task *ollamaDownloadTask) {
	s.updateTask(task.ID, func(t *ollamaDownloadTask) {
		t.Status = "downloading"
		t.Message = "开始下载"
	})

	payload := map[string]interface{}{
		"model":  task.ModelName,
		"stream": true,
	}
	raw, _ := json.Marshal(payload)
	req1, err := http.NewRequestWithContext(context.Background(), http.MethodPost, appendPath(s.ollamaBaseURL(), "pull"), bytes.NewReader(raw))
	if err != nil {
		s.failTask(task.ID, err)
		return
	}
	req1.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req1)
	if err != nil {
		s.failTask(task.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		s.failTask(task.ID, fmt.Errorf("下载失败: %s", strings.TrimSpace(string(body))))
		return
	}

	decoder := json.NewDecoder(resp.Body)
	for {
		var event map[string]interface{}
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			s.failTask(task.ID, err)
			return
		}

		statusText := stringValue(event["status"])
		total := float64Value(event["total"])
		completed := float64Value(event["completed"])
		progress := 0.0
		if total > 0 {
			progress = (completed / total) * 100
		}

		s.updateTask(task.ID, func(t *ollamaDownloadTask) {
			if progress > t.Progress {
				t.Progress = progress
			}
			if statusText != "" {
				t.Message = statusText
			}
		})

		if statusText == "success" {
			now := time.Now()
			s.updateTask(task.ID, func(t *ollamaDownloadTask) {
				t.Status = "completed"
				t.Progress = 100
				t.Message = "下载完成"
				t.EndTime = &now
			})
			return
		}
		if errorText := stringValue(event["error"]); errorText != "" {
			s.failTask(task.ID, fmt.Errorf("%s", errorText))
			return
		}
	}

	now := time.Now()
	s.updateTask(task.ID, func(t *ollamaDownloadTask) {
		t.Status = "completed"
		t.Progress = 100
		t.Message = "下载完成"
		t.EndTime = &now
	})
}

// updateTask 线程安全地更新下载任务字段。
func (s *modelService) updateTask(taskID string, fn func(task *ollamaDownloadTask)) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	if task, ok := s.tasks[taskID]; ok {
		fn(task)
	}
}

// failTask 将下载任务标记为失败。
func (s *modelService) failTask(taskID string, err error) {
	now := time.Now()
	s.updateTask(taskID, func(task *ollamaDownloadTask) {
		task.Status = "failed"
		task.Message = err.Error()
		task.EndTime = &now
	})
}

// toDownloadTaskResponse 将内部任务结构转为 API 响应。
func (s *modelService) toDownloadTaskResponse(task *ollamaDownloadTask) *dto.DownloadTaskResponse {
	resp := &dto.DownloadTaskResponse{
		ID:        task.ID,
		ModelName: task.ModelName,
		Status:    task.Status,
		Progress:  task.Progress,
		Message:   task.Message,
		StartTime: task.StartTime.Format(time.RFC3339),
	}
	if task.EndTime != nil {
		resp.EndTime = task.EndTime.Format(time.RFC3339)
	}
	return resp
}

// ---------- Ollama 底层调用 ----------

// callOllamaEmbed 调用 Ollama embed API 生成向量。
func (s *modelService) callOllamaEmbed(modelName, input string) ([]float64, map[string]interface{}, error) {
	payload := map[string]interface{}{
		"model": modelName,
		"input": input,
	}
	status, body, err := s.doJSONRequest(http.MethodPost, appendPath(s.ollamaBaseURL(), "embed"), payload, nil)
	if err != nil {
		return nil, map[string]interface{}{}, err
	}
	if status < 200 || status >= 300 {
		return nil, body, fmt.Errorf("%s", extractErrorMessage(body, status))
	}
	return extractEmbeddingVector(body), body, nil
}

// callOllamaChat 调用 Ollama chat API 生成对话回复，支持 vision 图片。
func (s *modelService) callOllamaChat(modelName, input string, options map[string]interface{}, fileHeader *multipart.FileHeader, withVision bool) (map[string]interface{}, string, error) {
	message := map[string]interface{}{
		"role":    "user",
		"content": input,
	}
	if withVision && fileHeader != nil {
		data, err := fileHeaderBytes(fileHeader)
		if err != nil {
			return map[string]interface{}{}, "", err
		}
		message["images"] = []string{base64.StdEncoding.EncodeToString(data)}
	}

	payload := map[string]interface{}{
		"model":    modelName,
		"messages": []map[string]interface{}{message},
		"stream":   false,
	}
	ollamaOptions := map[string]interface{}{}
	if temperature, ok := options["temperature"]; ok {
		ollamaOptions["temperature"] = temperature
	}
	if topP, ok := options["top_p"]; ok {
		ollamaOptions["top_p"] = topP
	}
	if maxTokens, ok := options["max_tokens"]; ok {
		ollamaOptions["num_predict"] = maxTokens
	}
	if len(ollamaOptions) > 0 {
		payload["options"] = ollamaOptions
	}

	status, body, err := s.doJSONRequest(http.MethodPost, appendPath(s.ollamaBaseURL(), "chat"), payload, nil)
	if err != nil {
		return map[string]interface{}{}, "", err
	}
	if status < 200 || status >= 300 {
		return body, "", fmt.Errorf("%s", extractErrorMessage(body, status))
	}
	answer := ""
	if msg, ok := body["message"].(map[string]interface{}); ok {
		answer = stringValue(msg["content"])
	}
	return body, answer, nil
}

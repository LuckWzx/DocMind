package service

import (
	"context"
	"mime/multipart"

	req "docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
)

// ModelService 模型管理服务接口
type ModelService interface {
	// BackfillContextWindows 存量模型 context_window 补全（启动时后台调用），返回补全成功数
	BackfillContextWindows(ctx context.Context) (int, error)
	// ListMissingContextWindows 上下文大小缺失模型清单（供定期补足内置映射表）
	ListMissingContextWindows() ([]*dto.ModelContextWindowMissingResponse, error)
	CreateModel(userID uint, request *req.UpsertModelRequest) (*dto.ModelResponse, error)
	ListModels(userID uint, modelType string) ([]*dto.ModelResponse, error)
	GetModel(userID uint, id uint) (*dto.ModelResponse, error)
	UpdateModel(userID uint, id uint, request *req.UpsertModelRequest) (*dto.ModelResponse, error)
	DeleteModel(userID uint, id uint) error
	PutModelCredentials(userID uint, id uint, request *req.PutModelCredentialsRequest) (*dto.ModelCredentialsResponse, error)
	DeleteModelCredentialField(userID uint, id uint, field string) (*dto.ModelCredentialsResponse, error)
	ListProviders(modelType string) []*dto.ModelProviderOptionResponse
	SaveDocMindCloudCredentials(appID, appSecret string) error
	GetDocMindCloudStatus() (*dto.DocMindCloudStatusResponse, error)
	CheckRemoteModel(request *req.ModelTestRequest) (map[string]interface{}, error)
	TestEmbeddingModel(request *req.ModelTestRequest) (map[string]interface{}, error)
	CheckRerankModel(request *req.ModelTestRequest) (map[string]interface{}, error)
	CheckASRModel(request *req.ModelTestRequest) (map[string]interface{}, error)

	GetOllamaStatus() (*dto.OllamaStatusResponse, error)
	ListOllamaModels() ([]*dto.OllamaModelInfoResponse, error)
	CheckOllamaModels(models []string) (map[string]bool, error)
	DownloadOllamaModel(modelName string) (*dto.DownloadTaskResponse, error)
	GetOllamaDownloadProgress(taskID string) (*dto.DownloadTaskResponse, error)
	ListOllamaDownloadTasks() ([]*dto.DownloadTaskResponse, error)
	DebugModel(userID uint, id uint, input string, documents []string, options map[string]interface{}, fileHeader *multipart.FileHeader) (*dto.ModelDebugResult, error)
	EmbedText(userID uint, modelRef string, input string) ([]float32, error)
}

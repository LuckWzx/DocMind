package service

import (
	"mime/multipart"

	req "docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
)

// ModelService 模型管理服务接口
type ModelService interface {
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

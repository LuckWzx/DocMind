package service

import (
	"mime/multipart"

	req "docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
)

// ModelService 模型管理服务接口
type ModelService interface {
	CreateModel(request *req.UpsertModelRequest) (*dto.ModelResponse, error)
	ListModels(modelType string) ([]*dto.ModelResponse, error)
	GetModel(id uint) (*dto.ModelResponse, error)
	UpdateModel(id uint, request *req.UpsertModelRequest) (*dto.ModelResponse, error)
	DeleteModel(id uint) error
	PutModelCredentials(id uint, request *req.PutModelCredentialsRequest) (*dto.ModelCredentialsResponse, error)
	DeleteModelCredentialField(id uint, field string) (*dto.ModelCredentialsResponse, error)
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
	DebugModel(id uint, input string, documents []string, options map[string]interface{}, fileHeader *multipart.FileHeader) (*dto.ModelDebugResult, error)
}

package service

import (
	"time"

	"docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	pkgerrors "docmind/pkg/errors"
	"docmind/pkg/logger"
)

// recordMissingContextWindow 记录上下文大小缺失的模型：
// 厂商接口未返回元数据且内置映射表未命中时写入，供后续定期补足映射表。
func (s *modelService) recordMissingContextWindow(model *entity.Model, reason string) {
	if model == nil || model.ID == 0 {
		return
	}
	record := &entity.ModelContextWindowMissing{
		UserID:    model.UserID,
		ModelID:   model.ID,
		ModelName: model.Name,
		Provider:  model.Parameters.Provider,
		BaseURL:   model.Parameters.BaseURL,
		Source:    model.Source,
		Type:      model.Type,
		Reason:    reason,
	}
	if err := s.missingRepo.Upsert(record); err != nil {
		logger.Warnf("[ModelContextWindow] 模型 %s 缺失记录写入失败: %v", model.Name, err)
	}
}

// clearMissingContextWindow 清除指定模型的缺失记录（上下文已确定或模型已删除）。
func (s *modelService) clearMissingContextWindow(modelID uint) {
	if err := s.missingRepo.ClearByModelID(modelID); err != nil {
		logger.Warnf("[ModelContextWindow] 模型 %d 缺失记录清理失败: %v", modelID, err)
	}
}

// ListMissingContextWindows 返回上下文大小缺失的模型清单（供定期补足内置映射表）。
func (s *modelService) ListMissingContextWindows() ([]*response.ModelContextWindowMissingResponse, error) {
	records, err := s.missingRepo.ListAll()
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询上下文大小缺失清单失败", err)
	}
	result := make([]*response.ModelContextWindowMissingResponse, 0, len(records))
	for _, r := range records {
		result = append(result, &response.ModelContextWindowMissingResponse{
			ID:        r.ID,
			ModelID:   r.ModelID,
			ModelName: r.ModelName,
			Provider:  r.Provider,
			BaseURL:   r.BaseURL,
			Source:    r.Source,
			Type:      r.Type,
			Reason:    r.Reason,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}

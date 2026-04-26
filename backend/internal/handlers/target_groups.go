package handlers

import (
	"context"

	"codex3/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Server) applyTargetGroupFilter(ctx context.Context, query *gorm.DB, tenantID uuid.UUID, groupID uuid.UUID) *gorm.DB {
	subQuery := s.db.WithContext(ctx).
		Model(&models.TargetGroupBinding{}).
		Select("target_id").
		Where("group_id = ?", groupID)
	if tenantID != uuid.Nil {
		subQuery = subQuery.Where("tenant_id = ?", tenantID)
	}

	return query.Where("group_id = ? OR id IN (?)", groupID, subQuery)
}

func (s *Server) loadTargetGroupIDs(ctx context.Context, tenantID uuid.UUID, targets []models.Target) (map[uuid.UUID][]string, error) {
	groupIDsByTarget := make(map[uuid.UUID][]string, len(targets))
	if len(targets) == 0 {
		return groupIDsByTarget, nil
	}

	targetIDs := make([]uuid.UUID, 0, len(targets))
	seenByTarget := make(map[uuid.UUID]map[string]struct{}, len(targets))
	for _, target := range targets {
		targetIDs = append(targetIDs, target.ID)
		seenByTarget[target.ID] = map[string]struct{}{}
		if target.GroupID != nil {
			groupID := target.GroupID.String()
			groupIDsByTarget[target.ID] = append(groupIDsByTarget[target.ID], groupID)
			seenByTarget[target.ID][groupID] = struct{}{}
		}
	}

	var bindings []models.TargetGroupBinding
	query := s.db.WithContext(ctx).Where("target_id IN ?", targetIDs)
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Order("created_at asc").Find(&bindings).Error; err != nil {
		return nil, err
	}

	for _, binding := range bindings {
		groupID := binding.GroupID.String()
		if _, exists := seenByTarget[binding.TargetID][groupID]; exists {
			continue
		}
		groupIDsByTarget[binding.TargetID] = append(groupIDsByTarget[binding.TargetID], groupID)
		seenByTarget[binding.TargetID][groupID] = struct{}{}
	}

	return groupIDsByTarget, nil
}

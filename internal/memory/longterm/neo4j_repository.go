package longterm

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"docmind/pkg/config"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// schema 初始化查询（幂等，仅首次创建约束）
const (
	episodeConstraintQuery = "CREATE CONSTRAINT episode_id_unique IF NOT EXISTS FOR (e:Episode) REQUIRE e.id IS UNIQUE"
	entityConstraintQuery  = "CREATE CONSTRAINT entity_name_user_unique IF NOT EXISTS FOR (n:Entity) REQUIRE (n.name, n.user_id) IS UNIQUE"
)

// NewNeo4jDriver 创建 Neo4j 驱动并校验连通性。
// enabled=false 时返回 nil（上层整体降级跳过，不阻塞主流程）。
func NewNeo4jDriver(cfg config.Neo4jConfig) (neo4j.DriverWithContext, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.URI == "" {
		return nil, fmt.Errorf("neo4j.uri 为空")
	}
	driver, err := neo4j.NewDriverWithContext(cfg.URI, neo4j.BasicAuth(cfg.Username, cfg.Password, ""))
	if err != nil {
		return nil, fmt.Errorf("创建 Neo4j 驱动失败: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := driver.VerifyConnectivity(ctx); err != nil {
		_ = driver.Close(ctx)
		return nil, fmt.Errorf("Neo4j 连通性校验失败: %w", err)
	}
	return driver, nil
}

type neo4jMemoryRepository struct {
	driver neo4j.DriverWithContext
}

// NewNeo4jMemoryRepository 创建 Neo4j 图仓储并幂等初始化 Schema。
// driver 为 nil 时返回 nil（调用方按降级处理）。
func NewNeo4jMemoryRepository(driver neo4j.DriverWithContext) (MemoryRepository, error) {
	if driver == nil {
		return nil, nil
	}
	repo := &neo4jMemoryRepository{driver: driver}
	if err := repo.initSchema(context.Background()); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *neo4jMemoryRepository) IsAvailable() bool {
	return r != nil && r.driver != nil
}

// initSchema 幂等创建唯一约束（启动时执行，失败则整体不可用）
func (r *neo4jMemoryRepository) initSchema(ctx context.Context) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	for _, q := range []string{episodeConstraintQuery, entityConstraintQuery} {
		if _, err := session.Run(ctx, q, nil); err != nil {
			return fmt.Errorf("初始化 Neo4j 约束失败: %w", err)
		}
	}
	return nil
}

func (r *neo4jMemoryRepository) SaveEpisode(ctx context.Context, episode *Episode, entities []*Entity, relations []*Relationship) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// 1. Episode 节点（user_id/session_id 存字符串，规避 uint 与驱动 int64 的类型转换）
		if _, err := tx.Run(ctx, `
			MERGE (e:Episode {id: $id})
			SET e.user_id = $user_id,
				e.session_id = $session_id,
				e.summary = $summary,
				e.created_at = $created_at
		`, map[string]any{
			"id":         episode.ID,
			"user_id":    formatUint(episode.UserID),
			"session_id": formatUint(episode.SessionID),
			"summary":    episode.Summary,
			"created_at": episode.CreatedAt.Format(time.RFC3339),
		}); err != nil {
			return nil, fmt.Errorf("创建 Episode 失败: %w", err)
		}

		// 2. 实体节点 + Episode -[:MENTIONS]-> Entity（按 (name, user_id) 唯一，多租户隔离）
		for _, entity := range entities {
			if _, err := tx.Run(ctx, `
				MERGE (n:Entity {name: $name, user_id: $user_id})
				SET n.type = $type, n.description = $description
				WITH n
				MATCH (e:Episode {id: $episode_id})
				MERGE (e)-[:MENTIONS]->(n)
			`, map[string]any{
				"name":        entity.Title,
				"user_id":     formatUint(episode.UserID),
				"type":        entity.Type,
				"description": entity.Description,
				"episode_id":  episode.ID,
			}); err != nil {
				return nil, fmt.Errorf("创建实体 %s 失败: %w", entity.Title, err)
			}
		}

		// 3. 实体间关系 RELATED_TO（同源同目标只保留一条边，属性更新为最新）
		for _, rel := range relations {
			if _, err := tx.Run(ctx, `
				MATCH (s:Entity {name: $source, user_id: $user_id})
				MATCH (t:Entity {name: $target, user_id: $user_id})
				MERGE (s)-[r:RELATED_TO]->(t)
				SET r.description = $description, r.weight = $weight
			`, map[string]any{
				"source":      rel.Source,
				"target":      rel.Target,
				"user_id":     formatUint(episode.UserID),
				"description": rel.Description,
				"weight":      rel.Weight,
			}); err != nil {
				return nil, fmt.Errorf("创建关系 %s->%s 失败: %w", rel.Source, rel.Target, err)
			}
		}
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("保存记忆片段失败: %w", err)
	}
	return nil
}

func (r *neo4jMemoryRepository) FindRelatedEpisodes(ctx context.Context, userID uint, keywords []string, limit int) ([]*Episode, error) {
	if len(keywords) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = DefaultRetrieveLimit
	}
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, `
			MATCH (e:Episode)-[:MENTIONS]->(n:Entity)
			WHERE e.user_id = $user_id AND n.name IN $keywords
			RETURN DISTINCT e
			ORDER BY e.created_at DESC
			LIMIT $limit
		`, map[string]any{
			"user_id":  formatUint(userID),
			"keywords": keywords,
			"limit":    limit,
		})
		if err != nil {
			return nil, err
		}
		var episodes []*Episode
		for res.Next(ctx) {
			record := res.Record()
			node, ok := record.Get("e")
			if !ok {
				continue
			}
			epNode, ok := node.(neo4j.Node)
			if !ok {
				continue
			}
			episodes = append(episodes, episodeFromNode(epNode))
		}
		return episodes, res.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("检索记忆片段失败: %w", err)
	}
	if result == nil {
		return nil, nil
	}
	return result.([]*Episode), nil
}

func (r *neo4jMemoryRepository) Close(ctx context.Context) error {
	if !r.IsAvailable() {
		return nil
	}
	return r.driver.Close(ctx)
}

// episodeFromNode Neo4j 节点 → Episode（属性均为字符串存储，读取时解析）
func episodeFromNode(node neo4j.Node) *Episode {
	ep := &Episode{}
	if v, ok := node.Props["id"]; ok {
		ep.ID, _ = v.(string)
	}
	if v, ok := node.Props["user_id"]; ok {
		if s, ok := v.(string); ok {
			ep.UserID, _ = parseUint(s)
		}
	}
	if v, ok := node.Props["session_id"]; ok {
		if s, ok := v.(string); ok {
			ep.SessionID, _ = parseUint(s)
		}
	}
	if v, ok := node.Props["summary"]; ok {
		ep.Summary, _ = v.(string)
	}
	if v, ok := node.Props["created_at"]; ok {
		if s, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				ep.CreatedAt = t
			}
		}
	}
	return ep
}

func formatUint(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

func parseUint(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	return uint(v), err
}

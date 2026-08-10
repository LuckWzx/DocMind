package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"docmind/internal/memory/longterm"
	"docmind/pkg/config"
	"docmind/pkg/logger"
)

// 长期记忆 demo：验证 Neo4j 知识图谱的落图与检索链路（不依赖 LLM，直接调用仓储层）。
//
// 前置条件：
//  1. Neo4j 已部署且 configs/config.yaml 中 neo4j.enabled=true、uri/账号密码正确
//  2. 运行：go run cmd/longtermdemo/main.go
//
// 场景：保存一条含实体/关系的记忆片段 → 用关键词检索 → 命中并打印 Episode。
func main() {
	_ = logger.InitDefault()

	fmt.Println("=== 长期记忆 demo（Neo4j 知识图谱）===")

	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}
	if !cfg.Neo4j.Enabled {
		fmt.Println("neo4j.enabled=false，请先在 configs/config.yaml 中启用并配置连接")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 创建驱动（连通性校验）
	driver, err := longterm.NewNeo4jDriver(cfg.Neo4j)
	if err != nil {
		fmt.Printf("Neo4j 连接失败: %v\n", err)
		os.Exit(1)
	}
	defer driver.Close(ctx)

	// 2. 创建仓储（幂等初始化唯一约束）
	repo, err := longterm.NewNeo4jMemoryRepository(driver)
	if err != nil {
		fmt.Printf("Neo4j 仓储初始化失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Neo4j 连接成功，Schema 约束已就绪")

	// 3. 保存记忆片段（手动构造，模拟 LLM 提取结果）
	episode := &longterm.Episode{
		ID:        fmt.Sprintf("demo-episode-%d", time.Now().UnixMilli()),
		UserID:    1,
		SessionID: 1,
		Summary:   "用户讨论了 DocMind 项目的长期记忆设计，决定使用 Neo4j 图数据库存储知识图谱。",
		CreatedAt: time.Now(),
	}
	entities := []*longterm.Entity{
		{Title: "DocMind", Type: "Project", Description: "知识库问答系统"},
		{Title: "Neo4j", Type: "Technology", Description: "图数据库"},
		{Title: "长期记忆", Type: "Concept", Description: "跨会话记忆机制"},
	}
	relations := []*longterm.Relationship{
		{Source: "DocMind", Target: "Neo4j", Description: "使用", Weight: 0.9},
		{Source: "DocMind", Target: "长期记忆", Description: "实现", Weight: 0.8},
	}
	if err := repo.SaveEpisode(ctx, episode, entities, relations); err != nil {
		fmt.Printf("落图失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("落图成功: %s（实体 %d 个，关系 %d 条）\n", episode.Summary, len(entities), len(relations))

	// 4. 关键词检索（模拟用户问"上次讨论的 DocMind 长期记忆"提取出的关键词）
	keywords := []string{"DocMind", "长期记忆"}
	episodes, err := repo.FindRelatedEpisodes(ctx, 1, keywords, 5)
	if err != nil {
		fmt.Printf("检索失败: %v\n", err)
		os.Exit(1)
	}
	if len(episodes) == 0 {
		fmt.Println("检索未命中（预期命中断言）")
		os.Exit(1)
	}
	fmt.Printf("检索命中 %d 条 Episode：\n", len(episodes))
	for _, ep := range episodes {
		fmt.Printf("  - [%s] %s\n", ep.CreatedAt.Format("2006-01-02"), ep.Summary)
	}

	fmt.Println("=== demo 通过：落图 → 检索链路正常 ===")
}

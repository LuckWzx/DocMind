package main

import (
	"fmt"

	"docmind/internal/app"
)

// @title           DocMind API
// @version         1.0.0
// @description     智能知识管理系统 - 将文档转化为可查询、可推理、持续进化的知识资产
// @termsOfService  http://swagger.io/terms/

// @contact.name   DocMind Team
// @contact.email  support@docmind.io

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:3888
// @BasePath  /api/v1

// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description 输入 Bearer {token}

func main() {
	// 创建应用实例
	application := app.NewApp()

	// 初始化应用
	if err := application.Initialize(); err != nil {
		panic(fmt.Sprintf("应用初始化失败: %v", err))
	}

	// 运行应用
	application.Run()
}

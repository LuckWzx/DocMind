package main

import (
	"fmt"

	"docmind/internal/app"
)

// 构建期注入变量：scripts/build.sh 通过 -ldflags 覆盖（Version/BuildTime/GoVersion/CommitID），
// 本地 go run / go build 时保持默认值；未注入的字段在系统信息接口中回退为配置或运行时取值
var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	GoVersion = "unknown"
	CommitID  = "unknown"
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
	// 创建应用实例（携带构建期版本信息，供系统信息接口展示）
	application := app.NewApp(app.BuildInfo{
		Version:   Version,
		CommitID:  CommitID,
		BuildTime: BuildTime,
	})

	// 初始化应用
	if err := application.Initialize(); err != nil {
		panic(fmt.Sprintf("应用初始化失败: %v", err))
	}

	// 运行应用
	application.Run()
}

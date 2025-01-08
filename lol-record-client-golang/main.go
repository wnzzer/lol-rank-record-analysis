package main

import (
	"github.com/gin-gonic/gin"
	"lol-record-analysis/api"
	"lol-record-analysis/api/handlers"
)

func main() {

	// 创建 Gin 路由器
	r := gin.Default()
	r.Use(handlers.Cors())

	// 初始化路由
	api.InitRoutes(r)

	// 启动服务
	r.Run(":11451") // 在 11451 端口上运行
}
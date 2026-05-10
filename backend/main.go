package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lumipluse-backend/internal/checker"
	"lumipluse-backend/internal/config"
	h "lumipluse-backend/internal/handler/http"
	"lumipluse-backend/internal/pkg/utils"
	"lumipluse-backend/internal/repository/sqlite"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

const Version = "0.1.1"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("LumiPulse version %s\n", Version)
			os.Exit(0)
		default:
			fmt.Printf("未知的参数: %s\n", os.Args[1])
			fmt.Printf("使用 --version 或 -v 查看版本信息\n")
			os.Exit(1)
		}
	}

	gin.SetMode(gin.ReleaseMode)

	time.Local = time.FixedZone("CST", 8*3600)

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("无法加载配置: %v", err)
	}

	dbPath := "./data/data.db"
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("无法创建数据库目录: %v", err)
	}

	db, err := sqlx.Connect("sqlite", dbPath)
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	if err := sqlite.InitSchema(db); err != nil {
		log.Fatalf("初始化表结构失败: %v", err)
	}

	utils.InitSettingsDB(db)
	repo := sqlite.NewRepository(db)
	handler := &h.Handler{Repo: repo, Version: Version}

	// Start health checker
	hc := checker.New(repo)
	hc.Start(context.Background())

	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		allowOriginStr := utils.GetSetting("allow_origin")
		allowedOrigins := strings.Split(allowOriginStr, ",")
		for i := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
		}

		isAllowed := false
		for _, o := range allowedOrigins {
			if o == origin {
				isAllowed = true
				break
			}
		}

		if isAllowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	h.RegisterRoutes(r, handler)

	r.Static("/assets", "./public/assets")
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/api") {
			c.File("./public/index.html")
		}
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	fmt.Printf("--- LumiPulse Status Page ---\n")
	fmt.Printf("监听地址: %s\n", addr)
	fmt.Printf("数据库路径: %s\n", dbPath)
	fmt.Printf("版本: %s\n", Version)

	if err := r.Run(addr); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

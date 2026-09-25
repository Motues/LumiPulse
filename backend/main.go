package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
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

const Version = "0.1.11"

// sqliteDSN 构造带 pragma 的 SQLite 连接串。
// 说明：busy_timeout / foreign_keys 是「每连接」设置，必须写进 DSN 才能对
// 连接池里的每个连接生效；WAL 让读不阻塞写，避免检查器与接口并发写时拿不到锁。
func sqliteDSN(path string) string {
	return "file:" + path +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=foreign_keys(1)"
}

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
		utils.Fatal("无法加载配置: %v", err)
	}

	dbPath := "./data/data.db"
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		utils.Fatal("无法创建数据库目录: %v", err)
	}

	db, err := sqlx.Connect("sqlite", sqliteDSN(dbPath))
	if err != nil {
		utils.Fatal("数据库连接失败: %v", err)
	}
	defer db.Close()
	// 少量连接 + busy_timeout 即可；WAL 下读不会阻塞写
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(0)

	if err := sqlite.InitSchema(db); err != nil {
		utils.Fatal("初始化表结构失败: %v", err)
	}

	utils.InitSettingsDB(db)
	// 设置项读多写少（CORS 中间件每个请求都会读），启动时灌入内存缓存
	utils.WarmupSettings()
	repo := sqlite.NewRepository(db)
	handler := &h.Handler{Repo: repo, Version: Version}

	// 收到 SIGINT / SIGTERM 时取消该 context，用来停机；
	// 检查器与 HTTP 服务都挂在它上面，容器滚动更新时不会硬中断在途探测与写库。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start health checker
	hc := checker.New(repo, cfg.InsecureSkipVerify, cfg.ProbeConcurrency())
	// 检查器会自动创建/解决事件并改动服务状态，需要同步失效公开页缓存
	hc.SetOnDataChange(handler.InvalidateSummary)
	// 每日收尾：按周期发送月报与周报邮件（各自自检触发条件）
	hc.SetOnDailyMaintenance(handler.SendScheduledReportsIfDue)
	hc.Start(ctx)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cacheControlMiddleware())

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
			// 响应随 Origin 变化，必须声明，否则反代/浏览器可能缓存错版本
			c.Writer.Header().Add("Vary", "Origin")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// 动态接口（JSON / XML）启用 gzip，静态资源交给反向代理处理，避免 Range/Content-Length 冲突
	r.Use(gzipMiddleware())

	h.RegisterRoutes(r, handler)

	r.Static("/assets", "./public/assets")
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") {
			// 未匹配的接口路径必须返回 JSON，否则前端 res.json() 会抛解析错误
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Not found"})
			return
		}
		c.File("./public/index.html")
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	fmt.Printf("--- LumiPulse Status Page ---\n")
	fmt.Printf("监听地址: %s\n", addr)
	fmt.Printf("数据库路径: %s\n", dbPath)
	fmt.Printf("版本: %s\n", Version)

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 在独立协程里监听：主协程等待退出信号，收到后先停止接受新连接、
	// 等在途请求结束，再停掉检查器并关闭数据库。
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			utils.Fatal("服务器启动失败: %v", err)
		}
	case <-ctx.Done():
		utils.Info("收到退出信号，开始停机")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		utils.Info("HTTP 服务停机超时: %v", err)
	}

	// 检查器可能正在探测或写库，等它自己收敛后再关闭连接
	hc.Stop()
	utils.Info("服务已退出")
}

// cacheControlMiddleware 为内容哈希的构建产物加上长缓存，index.html 保持不缓存。
func cacheControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/assets/") {
			// Vite 产物文件名带内容哈希，可以放心长缓存
			c.Writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if path == "/" || path == "/index.html" || !strings.Contains(path, ".") {
			// SPA 入口（含前端路由路径）不缓存，保证发版后立即生效
			c.Writer.Header().Set("Cache-Control", "no-cache")
		}
		c.Next()
	}
}

// gzipMiddleware 只压缩动态接口响应（JSON / RSS / Atom）。
// 静态资源走反向代理压缩，这里不碰，避免与 Range / Content-Length 冲突。
func gzipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		isDynamic := strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/feed/")
		if !isDynamic || c.Request.Method == http.MethodHead ||
			!strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}

		orig := c.Writer
		orig.Header().Add("Vary", "Accept-Encoding")
		orig.Header().Del("Content-Length")
		orig.Header().Set("Content-Encoding", "gzip")

		gw := &gzipWriter{ResponseWriter: orig, gz: gzip.NewWriter(orig)}
		c.Writer = gw

		// 用 defer 保证 panic 场景也会先还原 writer 并关闭 gzip，
		// 否则 gin.Recovery() 会写进已关闭的 gzip writer，产出损坏的响应体。
		defer func() {
			c.Writer = orig
			gw.gz.Close()
			if !gw.wrote {
				// 没有任何响应体写出（例如 handler panic），撤销 gzip 声明
				orig.Header().Del("Content-Encoding")
			}
		}()

		c.Next()
	}
}

// gzipWriter 包装 gin.ResponseWriter，把写出的字节流交给 gzip。
type gzipWriter struct {
	gin.ResponseWriter
	gz    *gzip.Writer
	wrote bool
}

func (g *gzipWriter) WriteHeader(code int) {
	g.wrote = true
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	g.wrote = true
	return g.gz.Write(b)
}

func (g *gzipWriter) WriteString(s string) (int, error) {
	g.wrote = true
	return g.gz.Write([]byte(s))
}

func (g *gzipWriter) Flush() {
	_ = g.gz.Flush()
	g.ResponseWriter.Flush()
}

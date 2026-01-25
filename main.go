package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/getlantern/systray"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"vinylfo/config"
	"vinylfo/controllers"
	"vinylfo/database"
	"vinylfo/discogs"
	"vinylfo/models"
	"vinylfo/routes"
	"vinylfo/utils"
)

//go:embed .env icons templates static
var embeddedFiles embed.FS

var db *gorm.DB
var playbackController *controllers.PlaybackController
var server *http.Server
var exitChan chan struct{}
var iconBytes []byte
var logFile *os.File
var fileOpenMutex sync.Mutex

func init() {
	setupFileLogging()

	config.SetEmbeddedFS(embeddedFiles)

	data, err := config.LoadEmbeddedIcon("icons/vinyl-icon.ico")
	if err != nil {
		data, err = config.LoadEmbeddedIcon("icons/vinyl-icon.png")
		if err != nil {
			log.Printf("Warning: No icon file found (tried icons/vinyl-icon.ico and icons/vinyl-icon.png)")
			return
		}
	}
	if len(data) == 0 {
		log.Printf("Warning: Icon file is empty")
		return
	}
	iconBytes = data
	log.Printf("Loaded icon: %d bytes", len(data))
}

func setupFileLogging() {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return
	}

	logPath := utils.GetTimestampedLogPath(logDir, "vinylfo")
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}

	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func serveEmbeddedStatic(embeddedFS embed.FS, subdir string) http.Handler {
	return http.StripPrefix("/"+subdir+"/", http.FileServer(http.FS(embeddedFS)))
}

func cleanupLogsOnStartup(db *gorm.DB) {
	var config models.AppConfig
	result := db.First(&config)
	if result.Error != nil {
		return
	}

	retentionCount := config.LogRetentionCount
	if retentionCount <= 0 {
		retentionCount = 10
	}

	deleted, err := utils.CleanupOldLogs(retentionCount, "logs")
	if err == nil && deleted > 0 {
		log.Printf("Cleaned up %d old log files (retention: %d)", deleted, retentionCount)
	}
}

func main() {
	log.Println("Vinylfo starting...")
	config.LoadEmbeddedEnv()
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found. Using embedded configuration.")
	}

	db, err = database.InitDB()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	db.Logger.LogMode(logger.Info)

	validationResult := discogs.ValidateOAuthConfig()
	if !validationResult.IsValid {
		log.Println("Warning: OAuth configuration has errors. OAuth functionality may not work correctly.")
	}
	discogs.PrintOAuthConfigSummary()

	playbackController = controllers.NewPlaybackController(db)

	utils.InitPKCE(db)
	utils.InitAuditLog(db)

	cleanupLogsOnStartup(db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go playbackController.SimulateTimer(ctx)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	staticFS, _ := fs.Sub(embeddedFiles, "static")
	r.GET("/static/*filepath", gin.WrapH(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))))

	iconsFS, _ := fs.Sub(embeddedFiles, "icons")
	r.GET("/icons/*filepath", gin.WrapH(http.StripPrefix("/icons/", http.FileServer(http.FS(iconsFS)))))

	tmpl := template.Must(template.ParseFS(embeddedFiles,
		"templates/header.html",
		"templates/footer.html",
		"templates/index.html",
		"templates/search.html",
		"templates/sync.html",
		"templates/settings.html",
		"templates/playback-dashboard.html",
		"templates/playlist.html",
		"templates/track-detail.html",
		"templates/album-detail.html",
		"templates/resolution-center.html",
		"templates/youtube.html",
		"templates/video-feed.html",
		"templates/album-art-feed.html",
		"templates/track-feed.html",
	))
	r.SetHTMLTemplate(tmpl)

	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index-page", nil)
	})

	r.GET("/player", func(c *gin.Context) {
		c.HTML(200, "playback-dashboard-page", nil)
	})

	r.GET("/playlist", func(c *gin.Context) {
		c.HTML(200, "playlist-page", nil)
	})

	r.GET("/track/:id", func(c *gin.Context) {
		c.HTML(200, "track-detail-page", nil)
	})

	r.GET("/album/:id", func(c *gin.Context) {
		c.HTML(200, "album-detail-page", nil)
	})

	r.GET("/settings", func(c *gin.Context) {
		c.HTML(200, "settings-page", nil)
	})

	r.GET("/sync", func(c *gin.Context) {
		c.HTML(200, "sync-page", nil)
	})

	r.GET("/search", func(c *gin.Context) {
		c.HTML(200, "search-page", nil)
	})

	r.GET("/resolution-center", func(c *gin.Context) {
		c.HTML(200, "resolution-center-page", nil)
	})

	r.GET("/youtube", func(c *gin.Context) {
		c.HTML(200, "youtube-page", nil)
	})

	routes.SetupRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server:", err)
		}
	}()

	go func() {
		time.Sleep(2 * time.Second)
		openBrowser()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go runSystray()

	select {
	case <-quit:
	case <-exitChan:
	}
	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.HTTP.ShutdownTimeout)
	defer shutdownCancel()

	cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Graceful database shutdown (includes WAL checkpoint for SQLite)
	if err := database.ShutdownDB(); err != nil {
		log.Printf("Error during database shutdown: %v", err)
	}

	log.Println("Server exited")
}

func runSystray() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Warning: Systray initialization failed: %v", r)
		}
	}()

	systray.Run(func() {
		if len(iconBytes) > 0 {
			systray.SetIcon(iconBytes)
			systray.SetTemplateIcon(iconBytes, iconBytes)
		}
		systray.SetTitle("Vinylfo")
		systray.SetTooltip("Vinylfo Server")

		mOpenLogs := systray.AddMenuItem("Open Log File", "View application logs")
		mOpenBrowser := systray.AddMenuItem("Open Web Interface", "Open web interface in browser")
		systray.AddSeparator()
		mExit := systray.AddMenuItem("Exit", "Exit application")

		go func() {
			for {
				select {
				case <-mOpenLogs.ClickedCh:
					go openLogFile()
				case <-mOpenBrowser.ClickedCh:
					go openBrowser()
				case <-mExit.ClickedCh:
					close(exitChan)
					systray.Quit()
				}
			}
		}()
	}, func() {
		log.Println("Systray exiting")
	})
}

func openLogFile() {
	logDir := "logs"
	files, _ := os.ReadDir(logDir)
	var latestFile os.FileInfo
	for _, f := range files {
		info, _ := f.Info()
		if strings.HasPrefix(info.Name(), "vinylfo_") && strings.HasSuffix(info.Name(), ".log") {
			if latestFile == nil || info.ModTime().After(latestFile.ModTime()) {
				latestFile = info
			}
		}
	}
	if latestFile != nil {
		openFile(filepath.Join(logDir, latestFile.Name()))
	}
}

func openBrowser() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	url := fmt.Sprintf("http://localhost:%s", port)
	if runtime.GOOS == "windows" {
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	} else if runtime.GOOS == "darwin" {
		exec.Command("open", url).Start()
	} else {
		exec.Command("xdg-open", url).Start()
	}
}

func openFile(path string) {
	fileOpenMutex.Lock()
	defer fileOpenMutex.Unlock()

	if runtime.GOOS == "windows" {
		openFileWindows(path)
	} else if runtime.GOOS == "darwin" {
		_ = exec.Command("open", path).Start()
	} else {
		_ = exec.Command("xdg-open", path).Start()
	}
}

func openFileWindows(path string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return
	}

	pathPtr, err := syscall.UTF16PtrFromString(absPath)
	if err != nil {
		return
	}

	shell32 := syscall.MustLoadDLL("shell32.dll")
	procShellExecute := shell32.MustFindProc("ShellExecuteW")

	_, _, _ = procShellExecute.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("open"))),
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		0,
		uintptr(1),
	)

	time.Sleep(100 * time.Millisecond)
}

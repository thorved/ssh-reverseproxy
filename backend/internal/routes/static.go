package routes

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func registerStaticRoutes(router *gin.Engine) {
	staticPath := resolveStaticPath()
	if staticPath == "" {
		return
	}

	router.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(staticPath, "index.html"))
	})

	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.URL.Path == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}

		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}

		if filePath, ok := resolveRequestedAsset(staticPath, c.Request.URL.Path); ok {
			c.File(filePath)
			return
		}

		if filePath, ok := resolveStatic404(staticPath); ok {
			c.File(filePath)
			return
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
	})
}

func resolveStaticPath() string {
	possiblePaths := []string{
		func() string {
			execPath, err := os.Executable()
			if err != nil {
				return ""
			}
			return filepath.Join(filepath.Dir(execPath), "frontend", "out")
		}(),
		"../frontend/out",
		"./frontend/out",
		"./../frontend/out",
	}

	for _, path := range possiblePaths {
		if path == "" {
			continue
		}

		indexPath := filepath.Join(path, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			return path
		}
	}

	return ""
}

func resolveRequestedAsset(staticPath, requestPath string) (string, bool) {
	cleanPath := strings.TrimPrefix(filepath.Clean("/"+requestPath), "/")
	if cleanPath == "." || cleanPath == "" {
		target := filepath.Join(staticPath, "index.html")
		return target, true
	}

	candidates := []string{
		filepath.Join(staticPath, cleanPath),
		filepath.Join(staticPath, cleanPath, "index.html"),
	}

	if filepath.Ext(cleanPath) == "" {
		candidates = append(candidates, filepath.Join(staticPath, cleanPath+".html"))
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}

		if info.IsDir() {
			indexPath := filepath.Join(candidate, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				return indexPath, true
			}
			continue
		}

		return candidate, true
	}

	return "", false
}

func resolveStatic404(staticPath string) (string, bool) {
	candidates := []string{
		filepath.Join(staticPath, "404.html"),
		filepath.Join(staticPath, "404", "index.html"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}

	return "", false
}

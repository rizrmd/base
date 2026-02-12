package spa

import (
	"context"
	"hash/fnv"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"

	"encore.dev"
	"encore.dev/beta/errs"
)

//encore:service
// Service serves the SPA - proxies to dev server in development,
// serves static files in production.
type Service struct {
	proxy  *httputil.ReverseProxy
	static http.Handler
}

func initService() (*Service, error) {
	s := &Service{}

	if encore.Meta().Environment.Type == "development" {
		// In development, proxy to the frontend dev server
		frontendPort := getFrontendPort()
		target, _ := url.Parse("http://localhost:" + strconv.Itoa(frontendPort))
		s.proxy = httputil.NewSingleHostReverseProxy(target)

		// Customize error handling for dev proxy
		s.proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Frontend dev server not reachable. Make sure it's running on port "+strconv.Itoa(frontendPort), http.StatusBadGateway)
		}
	} else {
		// In production, serve static files from frontend/build/client (React Router)
		distPath := "./frontend/build/client"
		if _, err := os.Stat(distPath); os.IsNotExist(err) {
			// Fallback to dist folder for other frameworks
			distPath = "./frontend/dist"
		}

		// Create file system for static files
		staticFS, err := fs.Sub(os.DirFS(distPath), ".")
		if err != nil {
			return nil, err
		}

		// SPA handler: serve static files, fall back to index.html for SPA routes
		fileServer := http.FileServer(http.FS(staticFS))
		s.static = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to serve the file directly
			path := r.URL.Path
			if path == "/" {
				path = "/index.html"
			}

			// Check if file exists
			if _, err := staticFS.Open(path[1:]); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}

			// For SPA routes, serve index.html
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		})
	}

	return s, nil
}

// getPortOffset returns a unique offset based on user identity + project path
// This ensures different projects by the same user get different ports
func getPortOffset() int {
	h := fnv.New32a()

	// Include user identity
	uid := os.Getuid()
	if uid == -1 {
		// Windows: hash the username
		h.Write([]byte(os.Getenv("USERNAME")))
	} else {
		// Unix: hash the UID
		h.Write([]byte(strconv.Itoa(uid)))
	}

	// Include project path for uniqueness across projects
	// Encore runs from apps/ directory, so get parent (project root)
	if cwd, err := os.Getwd(); err == nil {
		h.Write([]byte(cwd))
	}

	return int(h.Sum32() % 1000)
}

func getFrontendPort() int {
	// Check environment variable first (set by start binary)
	if port := os.Getenv("FRONTEND_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			return p
		}
	}

	// Fallback: calculate from user identity (same logic as start binary)
	return 5173 + getPortOffset()
}

// Serve handles all SPA routes.
//
//encore:api public raw path=/!base
func (s *Service) Serve(w http.ResponseWriter, req *http.Request) {
	if s.proxy != nil {
		s.proxy.ServeHTTP(w, req)
	} else if s.static != nil {
		s.static.ServeHTTP(w, req)
	} else {
		errs.HTTPError(w, &errs.Error{
			Code:    errs.Unavailable,
			Message: "SPA handler not configured",
		})
	}
}

// Health returns the health status of the SPA service.
//
//encore:api public path=/health
func (s *Service) Health(ctx context.Context) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok"}, nil
}

type HealthResponse struct {
	Status string `json:"status"`
}

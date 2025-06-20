package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"webscraper/usecase"

	"github.com/gorilla/mux"
)

type Server struct {
	port    string
	usecase *usecase.ScrapingUseCase
	router  *mux.Router
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewServer(port string, uc *usecase.ScrapingUseCase) *Server {
	s := &Server{port: port, usecase: uc, router: mux.NewRouter()} 
	s.setupRoutes()
	s.setupMiddleware()
	return s
}

func (s *Server) setupRoutes() {
	s.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./interface/static/"))))

	api := s.router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/scrape", s.scrapeHandler).Methods("POST")
	api.HandleFunc("/results", s.resultsHandler).Methods("GET")
	api.HandleFunc("/results/{id:[0-9]+}", s.resultHandler).Methods("GET")
	api.HandleFunc("/results/{id:[0-9]+}", s.deleteResultHandler).Methods("DELETE")
	api.HandleFunc("/health", s.healthHandler).Methods("GET")
	api.PathPrefix("/").HandlerFunc(s.notFoundHandler)

	s.router.HandleFunc("/", s.indexHandler).Methods("GET")
}

func (s *Server) setupMiddleware() {
	s.router.Use(s.loggingMiddleware, s.corsMiddleware, s.contentTypeMiddleware)
}

func (s *Server) Start() error {
	endpoints := []string{ 
		"GET  / - Web interface",
		"POST /api/scrape - Scrape URL",
		"GET  /api/results - Get all results",
		"GET  /api/results/{id} - Get specific result",
		"DELETE /api/results/{id} - Delete result",
		"GET  /api/health - Health check",
	}

	log.Printf("Server listening on port %s\nAvailable endpoints:\n  %s", s.port, strings.Join(endpoints, "\n  "))
	return http.ListenAndServe(":"+s.port, s.router)
}

func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./interface/templates/index.html")
}

func (s *Server) scrapeHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendErrorResponse(w, "Invalid JSON format", http.StatusBadRequest, err.Error())
		return
	}

	if req.URL = strings.TrimSpace(req.URL); req.URL == "" {
		s.sendErrorResponse(w, "URL is required", http.StatusBadRequest, "")
		return
	}

	parsedURL, err := url.ParseRequestURI(req.URL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		s.sendErrorResponse(w, "Invalid URL format", http.StatusBadRequest, "URL must include protocol (http:// or https://) and valid domain")
		return
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		s.sendErrorResponse(w, "Invalid URL scheme", http.StatusBadRequest, "Only HTTP and HTTPS protocols are supported")
		return
	}

	log.Printf("Scraping URL: %s", req.URL)
	result, err := s.usecase.ScrapeURL(req.URL)
	if err != nil {
		log.Printf("Error scraping URL %s: %v", req.URL, err)
		s.sendErrorResponse(w, "Failed to scrape URL", http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("Successfully scraped URL: %s (Status: %d, Words: %d)", req.URL, result.StatusCode, result.WordCount)
	s.sendSuccessResponse(w, "URL scraped successfully", result)
}

func (s *Server) resultsHandler(w http.ResponseWriter, r *http.Request) {
	results, err := s.usecase.GetAllResults()
	if err != nil {
		log.Printf("Error getting results: %v", err)
		s.sendErrorResponse(w, "Failed to retrieve results", http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("Retrieved %d scraping results", len(results))
	s.sendSuccessResponse(w, fmt.Sprintf("Retrieved %d results", len(results)), results)
}

func (s *Server) resultHandler(w http.ResponseWriter, r *http.Request) {
	id, err := s.parseID(r)
	if err != nil {
		s.sendErrorResponse(w, "Invalid ID format", http.StatusBadRequest, "ID must be a valid number")
		return
	}

	result, err := s.usecase.GetResult(id)
	if err != nil {
		log.Printf("Error getting result %d: %v", id, err)
		s.sendErrorResponse(w, "Failed to retrieve result", http.StatusInternalServerError, err.Error())
		return
	}

	if result == nil {
		s.sendErrorResponse(w, "Result not found", http.StatusNotFound, fmt.Sprintf("No result found with ID %d", id))
		return
	}

	log.Printf("Retrieved result ID: %d (%s)", id, result.URL)
	s.sendSuccessResponse(w, "Result retrieved successfully", result)
}

func (s *Server) deleteResultHandler(w http.ResponseWriter, r *http.Request) {
	id, err := s.parseID(r)
	if err != nil {
		s.sendErrorResponse(w, "Invalid ID format", http.StatusBadRequest, "ID must be a valid number")
		return
	}

	result, err := s.usecase.GetResult(id)
	if err != nil {
		log.Printf("Error checking result %d: %v", id, err)
		s.sendErrorResponse(w, "Failed to check result", http.StatusInternalServerError, err.Error())
		return
	}

	if result == nil {
		s.sendErrorResponse(w, "Result not found", http.StatusNotFound, fmt.Sprintf("No result found with ID %d", id))
		return
	}

	if err := s.usecase.DeleteResult(id); err != nil {
		log.Printf("Error deleting result %d: %v", id, err)
		s.sendErrorResponse(w, "Failed to delete result", http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("Deleted result ID: %d (%s)", id, result.URL)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status": "ok", "timestamp": time.Now().UTC().Format(time.RFC3339),
		"service": "webscraper", "version": "2.0",
	}
	s.sendSuccessResponse(w, "Service is healthy", health)
}

func (s *Server) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	s.sendErrorResponse(w, "Endpoint not found", http.StatusNotFound,
		fmt.Sprintf("The requested endpoint %s %s does not exist", r.Method, r.URL.Path))
}

// Helper methods
func (s *Server) parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
}

func (s *Server) sendErrorResponse(w http.ResponseWriter, message string, statusCode int, details string) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message, Message: details, Code: statusCode})
}

func (s *Server) sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	json.NewEncoder(w).Encode(SuccessResponse{Message: message, Data: data})
}

// Middleware
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(ww, r)
		log.Printf("%s %s - %d - %v - %s", r.Method, r.URL.Path, ww.statusCode, time.Since(start), r.RemoteAddr)
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) contentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
		}
		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

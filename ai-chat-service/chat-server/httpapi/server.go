package httpapi

import (
	"ai-chat-service/chat-server/storage"
	"ai-chat-service/pkg/config"
	"ai-chat-service/pkg/log"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	cnf        *config.Config
	log        log.ILogger
	qaStore    storage.IQAStore
	topicStore storage.ITopicStore
}

func NewServer(cnf *config.Config, logger log.ILogger, qaStore storage.IQAStore, topicStore storage.ITopicStore) *Server {
	return &Server{
		cnf:        cnf,
		log:        logger,
		qaStore:    qaStore,
		topicStore: topicStore,
	}
}

func (s *Server) Start(ctx context.Context) {
	if s.cnf == nil || !s.cnf.KBase.HTTPAPIEnabled {
		return
	}
	port := s.cnf.KBase.HTTPAPIPort
	if port <= 0 {
		port = 50056
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/kbase/health", s.handleHealth)
	mux.HandleFunc("/api/kbase/qa", s.handleQA)
	mux.HandleFunc("/api/kbase/topics", s.handleTopics)
	mux.HandleFunc("/api/kbase/export/qa", s.handleExportQA)

	addr := fmt.Sprintf("%s:%d", s.cnf.Server.IP, port)
	server := &http.Server{Addr: addr, Handler: withCORS(mux)}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	s.log.InfoF("qa internal api on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.log.Error(err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleQA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	entries, err := s.qaStore.ListAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	topicFilter := strings.TrimSpace(r.URL.Query().Get("topic"))
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		if e.Record == nil {
			continue
		}
		if topicFilter != "" && e.Record.TopicName != topicFilter {
			continue
		}
		out = append(out, map[string]any{
			"hash":       e.Hash,
			"question":   e.Record.Question,
			"answer":     e.Record.Answer,
			"topic_id":   e.Record.TopicID,
			"topic_name": e.Record.TopicName,
			"source":     e.Record.Source,
			"created_at": e.Record.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": len(out), "items": out})
}

func (s *Server) handleTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	topics, err := s.topicStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": len(topics), "topics": topics})
}

func (s *Server) handleExportQA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	entries, err := s.qaStore.ListAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := storage.EntriesToExportItems(entries)
	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "markdown"
	}
	byTopic := r.URL.Query().Get("by_topic") == "true" || r.URL.Query().Get("by_topic") == "1"
	switch format {
	case "json":
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=qa-export.json")
		w.Write(data)
	default:
		var content string
		if byTopic {
			content = storage.ExportMarkdownByTopic(items)
		} else {
			content = storage.ExportMarkdown(items)
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=qa-export.md")
		w.Write([]byte(content))
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg, "status": status})
}

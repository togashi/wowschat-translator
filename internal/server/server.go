package server

import (
	"context"
	"log"
	"net/http"

	"github.com/togashi/wowschat-translator/internal/translator"
)

const listenAddr = "127.0.0.1:5000"

type Server struct {
	tr         *translator.Translator
	targetLang string
	http       *http.Server
}

func New(tr *translator.Translator, targetLang string) *Server {
	s := &Server{
		tr:         tr,
		targetLang: targetLang,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/wowschat/", s.handle)
	s.http = &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}
	return s
}

// Start begins serving. Blocks until the server is closed.
func (s *Server) Start() error {
	log.Printf("listening on http://%s/wowschat/", listenAddr)
	if err := s.http.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	text := r.URL.Query().Get("text")
	if text == "" {
		return
	}

	log.Printf("received: %q", text)

	result, err := s.tr.Translate(text, s.targetLang)
	if err != nil {
		log.Printf("translation error: %v", err)
		return
	}

	if result == "" {
		log.Printf("skip: no translation needed")
	} else {
		log.Printf("translated: %q", result)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(result))
}

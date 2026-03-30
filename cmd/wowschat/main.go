package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kardianos/service"
	"github.com/togashi/wowschat/internal/config"
	"github.com/togashi/wowschat/internal/server"
	"github.com/togashi/wowschat/internal/translator"
)

var svcConfig = &service.Config{
	Name:        "WoWSChat",
	DisplayName: "WoWS Chat Translator",
	Description: "World of Warships in-game chat translation service via DeepL",
}

type program struct {
	srv *server.Server
}

func (p *program) Start(_ service.Service) error {
	go func() {
		if err := p.srv.Start(); err != nil {
			log.Printf("server error: %v", err)
		}
	}()
	return nil
}

func (p *program) Stop(_ service.Service) error {
	return p.srv.Shutdown(context.Background())
}

func main() {
	var (
		configFile = flag.String("config", "", "path to config file (default: config.yaml next to executable)")
		apiKey     = flag.String("api-key", "", "DeepL API key")
		targetLang = flag.String("target-lang", "", "target language code (e.g. JA, EN-US)")
		outputFmt  = flag.String("output-format", "", "translated output format (e.g. ({DetectedSourceLanguage}) {TranslatedText})")
	)
	flag.Parse()

	// Service management commands (install / uninstall / start / stop)
	if args := flag.Args(); len(args) > 0 {
		runServiceCommand(args[0])
		return
	}

	cfg, err := config.Load(*configFile, *apiKey, *targetLang, *outputFmt)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if cfg.APIKey == "" {
		log.Fatal("DeepL API key is required.\n" +
			"  Set via --api-key flag, WOWSCHAT_API_KEY env var, or api_key in config.yaml")
	}

	log.Printf("target language: %s", cfg.TargetLang)

	tr := translator.New(cfg.APIKey, cfg.OutputFormat)
	srv := server.New(tr, cfg.TargetLang)
	prg := &program{srv: srv}

	svc, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatalf("service init: %v", err)
	}

	if service.Interactive() {
		// Running interactively (not as a service)
		go func() {
			if err := srv.Start(); err != nil {
				log.Printf("server stopped: %v", err)
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit

		log.Println("shutting down...")
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("shutdown: %v", err)
		}
	} else {
		// Running as a Windows service
		if err := svc.Run(); err != nil {
			log.Fatalf("service run: %v", err)
		}
	}
}

func runServiceCommand(cmd string) {
	svc, err := service.New(&program{}, svcConfig)
	if err != nil {
		log.Fatalf("service: %v", err)
	}

	switch cmd {
	case "install":
		err = svc.Install()
	case "uninstall":
		err = svc.Uninstall()
	case "start":
		err = svc.Start()
	case "stop":
		err = svc.Stop()
	default:
		log.Fatalf("unknown command %q  (valid: install / uninstall / start / stop)", cmd)
	}

	if err != nil {
		log.Fatalf("%s: %v", cmd, err)
	}
	log.Printf("service %s: OK", cmd)
}

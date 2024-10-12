package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gpgenie/internal/config"
	"gpgenie/internal/database"
	"gpgenie/internal/key"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to config file")
	generateKeys := flag.Bool("generate-keys", false, "Generate GPG keys")
	exportTopKeys := flag.Int("export-top", 0, "Export top N keys by score")
	exportLowLetterCount := flag.Int("export-low-letter", 0, "Export N keys with lowest letter count")
	outputFile := flag.String("output", "exported_keys.csv", "Output file for exported keys")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.CloseDB(db)

	s := key.New(db, cfg)

	if *generateKeys {
		err = s.GenerateKeys()
		if err != nil {
			log.Fatalf("Failed to generate keys: %v", err)
		}
		log.Println("Finished generating GPG keys")
		return
	}

	if *exportTopKeys > 0 {
		err = s.ExportTopKeys(*exportTopKeys, *outputFile)
		if err != nil {
			log.Fatalf("Failed to export top keys: %v", err)
		}
		return
	}

	if *exportLowLetterCount > 0 {
		err = s.ExportLowLetterCountKeys(*exportLowLetterCount, *outputFile)
		if err != nil {
			log.Fatalf("Failed to export low letter count keys: %v", err)
		}
		return
	}

	log.Println("Processing completed successfully")

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	log.Println("Shutting down gracefully...")
	// Perform any cleanup here
	database.CloseDB(db)
}

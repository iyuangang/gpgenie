package key

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gpgenie/internal/config"
	"gpgenie/internal/logger"
)

type Scorer struct {
	db *sql.DB
	config *config.Config
	encryptor *Encryptor
}

func New(db *sql.DB, cfg *config.Config, encryptor *Encryptor) *Scorer {
	s := &Scorer{db: db, config: cfg, encryptor: encryptor}
	s.ensureTablesExist()
	return s
}

func (s *Scorer) ensureTablesExist() {
	var err error
	if s.config.Database.Type == "postgres" {
		_, err = s.db.Exec(`
			CREATE TABLE IF NOT EXISTS gpg_ed25519_keys (
				fingerprint VARCHAR(255) PRIMARY KEY,
				public_key TEXT,
				private_key TEXT,
				rl_score INT,
				il_score INT,
				dl_score INT,
				ml_score INT,
				score INT,
				letters_count INT
			)
		`)
	} else { // SQLite
		_, err = s.db.Exec(`
			CREATE TABLE IF NOT EXISTS gpg_ed25519_keys (
				fingerprint TEXT PRIMARY KEY,
				public_key TEXT,
				private_key TEXT,
				rl_score INTEGER,
				il_score INTEGER,
				dl_score INTEGER,
				ml_score INTEGER,
				score INTEGER,
				letters_count INTEGER
			)
		`)
	}
	if err != nil {
		logger.Logger.Fatalf("Failed to create gpg_ed25519_keys table: %v", err)
	}
}

func (s *Scorer) ExportTopKeys(limit int, outputFile string) error {
	query := `
		SELECT upper(SUBSTR(fingerprint, 25, 16)), score, letters_count, public_key, private_key
		FROM gpg_ed25519_keys
		WHERE 1=1
		ORDER BY score DESC, letters_count
		LIMIT $1
	`
	return s.exportKeys(query, limit, outputFile)
}

func (s *Scorer) ExportLowLetterCountKeys(limit int, outputFile string) error {
	query := `
		SELECT upper(SUBSTR(fingerprint, 25, 16)), score, letters_count, public_key, private_key
		FROM gpg_ed25519_keys
		WHERE letters_count < 5
		ORDER BY letters_count, score DESC
		LIMIT $1
	`
	return s.exportKeys(query, limit, outputFile)
}

func (s *Scorer) exportKeys(query string, limit int, outputFile string) error {
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString("Fingerprint,Score,LettersCount,PublicKey,PrivateKey\n")
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for rows.Next() {
		var fingerprint string
		var score, lettersCount int
		var publicKey, privateKey string
		err := rows.Scan(&fingerprint, &score, &lettersCount, &publicKey, &privateKey)
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Escape commas and newlines in the keys
		publicKey = strings.ReplaceAll(publicKey, "\n", "\\n")
		privateKey = strings.ReplaceAll(privateKey, "\n", "\\n")

		_, err = file.WriteString(fmt.Sprintf("%s,%d,%d,\"%s\",\"%s\"\n",
			fingerprint, score, lettersCount, publicKey, privateKey))
		if err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("row iteration failed: %w", err)
	}

	logger.Logger.Info("Exported " + strconv.Itoa(limit) + " keys to " + outputFile)
	return nil
}

func (s *Scorer) ExportKeyByFingerprint(lastSixteen string, outputDir string) error {
	query := `SELECT fingerprint, private_key FROM gpg_ed25519_keys WHERE fingerprint LIKE $1`
	row := s.db.QueryRow(query, "%"+strings.ToLower(lastSixteen))

	var fingerprint, encodedPrivateKey string
	err := row.Scan(&fingerprint, &encodedPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to find key: %w", err)
	}

	// Base64 解码私钥
	decodedPrivateKey, err := base64.StdEncoding.DecodeString(encodedPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to decode private key: %w", err)
	}

	// 创建输出文件
	outputFile := filepath.Join(outputDir, fingerprint+".enc")
	err = os.WriteFile(outputFile, decodedPrivateKey, 0600)
	if err != nil {
		return fmt.Errorf("failed to write encrypted private key to file: %w", err)
	}

	return nil
}

func (s *Scorer) ShowTopKeys(n int) error {
	query := `SELECT upper(substr(fingerprint, 25, 16)) as fingerprint, score, letters_count 
              FROM gpg_ed25519_keys 
              ORDER BY score DESC, letters_count
              LIMIT $1`
	
	return s.showKeys(query, n)
}

func (s *Scorer) ShowLowLetterCountKeys(n int) error {
	query := `SELECT upper(substr(fingerprint, 25, 16)) as fingerprint, score, letters_count 
              FROM gpg_ed25519_keys 
              ORDER BY letters_count, score DESC 
              LIMIT $1`
	
	return s.showKeys(query, n)
}

func (s *Scorer) showKeys(query string, n int) error {
	rows, err := s.db.Query(query, n)
	if err != nil {
		return fmt.Errorf("failed to query keys: %w", err)
	}
	defer rows.Close()

	fmt.Println("Fingerprint      Score  Letters Count")
	fmt.Println("---------------- ------ -------------")

	for rows.Next() {
		var fingerprint string
		var score, lettersCount int
		if err := rows.Scan(&fingerprint, &score, &lettersCount); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		fmt.Printf("%-16s %6d %13d\n", fingerprint, score, lettersCount)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating over rows: %w", err)
	}

	return nil
}

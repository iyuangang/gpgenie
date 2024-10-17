package key

import (
	"bytes"
	"encoding/base64"
	"errors"
	"gpgenie/internal/config"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
)

type Encryptor struct {
	Entity *openpgp.Entity
}

func NewEncryptor(cfg *config.KeyEncryptionConfig) (*Encryptor, error) {
	if cfg.PublicKeyPath == "" {
		return nil, errors.New("public_key_path is not provided in configuration")
	}

	pubKeyData, err := os.ReadFile(cfg.PublicKeyPath)
	if err != nil {
		return nil, err
	}

	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(pubKeyData))
	if err != nil {
		return nil, err
	}

	if len(entities) == 0 {
		return nil, errors.New("no public keys found in the provided file")
	}

	return &Encryptor{Entity: entities[0]}, nil
}

func (e *Encryptor) EncryptAndEncode(plaintext string) (string, error) {
	var buf bytes.Buffer
	w, err := openpgp.Encrypt(&buf, []*openpgp.Entity{e.Entity}, nil, nil, nil)
	if err != nil {
			return "", err
	}
	_, err = w.Write([]byte(plaintext))
	if err != nil {
			return "", err
	}
	err = w.Close()
	if err != nil {
			return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return encoded, nil
	//return buf.String(), nil
}

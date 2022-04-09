package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
)

func encryptBytes(data []byte, key string) ([]byte, error) {

	gcm, err := getGCMCLient(key)
	if err != nil {
		logger.Error("Error getting gcm client")
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())

	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		logger.Error("Error reading random bytes for nonce")
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

func decryptBytes(ciphertext []byte, key string) ([]byte, error) {
	gcm, err := getGCMCLient(key)
	if err != nil {
		logger.Error("Error getting gcm client")
		return nil, err
	}

	nonceSize := gcm.NonceSize()

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		logger.Error("Error decrypting ciphertext")
		return nil, err
	}

	return plaintext, nil
}

func getGCMCLient(key string) (cipher.AEAD, error) {
	c, err := aes.NewCipher([]byte(key))
	if err != nil {
		logger.Error("Error creating new cipher")
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		logger.Error("Error creating new AES GCM")
		return nil, err
	}
	return gcm, nil
}

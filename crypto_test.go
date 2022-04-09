package main

import (
	"testing"
)

func TestAESEncryption(t *testing.T) {
	originalText := "dan is the good man"
	encryptionKey := "this is an encryption key-------"

	cipherText, err := encryptBytes([]byte(originalText), encryptionKey)
	if err != nil {
		t.Fatal(err)
	}

	plainText, err := decryptBytes(cipherText, encryptionKey)
	if err != nil {
		t.Fatal(err)
	}

	if string(plainText) != originalText {
		t.Fatalf("Text not matchking %s != %s", string(plainText), originalText)
	}
}

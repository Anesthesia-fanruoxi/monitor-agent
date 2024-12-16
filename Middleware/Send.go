package Middleware

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
)

// 加密数据
func encrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// 压缩数据
func compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// 发送数据到指定的 URL
func SendData(url string, project string, data interface{}, key []byte, source string) error {
	// 创建要发送的数据结构
	sendData := SendDataType{
		PROJECT:   project,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
		SOURCE:    source,
	}

	// 序列化数据
	jsonData, err := json.Marshal(sendData)
	if err != nil {
		return err
	}
	log.Printf("原始数据大小: %d 字节，来源%s", len(jsonData), source) // 打印原始数据大小

	// 压缩数据
	compressedData, err := compress(jsonData)
	if err != nil {
		return err
	}
	log.Printf("压缩后数据大小: %d 字节，来源%s", len(compressedData), source) // 打印压缩后数据大小

	// 加密数据
	encryptedData, err := encrypt(compressedData, key)
	if err != nil {
		return err
	}
	log.Printf("加密后数据大小: %d 字节，来源%s", len(encryptedData), source) // 打印加密后数据大小

	// 发送加密压缩后的数据
	resp, err := http.Post(url, "application/octet-stream", bytes.NewBuffer(encryptedData))
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("关闭响应体失败: %v", err)
		}
	}(resp.Body)

	return nil
}

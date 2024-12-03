package Middleware

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
)

// ============================================ 发送数据
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

// 发送数据到指定的 URL
func SendData(url string, project string, data interface{}, key []byte, source string) error {
	// 创建要发送的数据结构
	sendData := SendDataType{
		PROJECT:   project,
		Data:      data, // 直接使用传入的数据，不再包装成切片
		Timestamp: time.Now().UnixMilli(),
		SOURCE:    source, // 设置 bulk 字段
	}

	jsonData, err := json.Marshal(sendData)
	if err != nil {
		return err
	}

	//log.Printf("加密前数据：%s", string(jsonData)) // 确保以字符串形式打印

	encryptedData, err := encrypt(jsonData, key)
	if err != nil {
		return err
	}

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	log.Printf("请求结果：%s", body)
	return nil
}

package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

func configuredAddress(flagValue string) (string, error) {
	addr := flagValue
	if port := os.Getenv("PORT"); port != "" && flagValue == defaultAddress {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("PORT 必须是有效端口号")
		}
		addr = net.JoinHostPort("127.0.0.1", port)
	}
	host, port, e := net.SplitHostPort(addr)
	if e != nil {
		return "", fmt.Errorf("监听地址无效: %w", e)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return "", fmt.Errorf("仅允许回环监听地址")
	}
	if strings.TrimSpace(port) == "" {
		return "", fmt.Errorf("监听端口不能为空")
	}
	return addr, nil
}

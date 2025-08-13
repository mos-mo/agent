package config

import (
	"os"
	"strconv"
)

// 설정 기본값 상수 정의
const (
	DEFAULT_SERVER_ADDR      = "localhost:50051" // 기본 gRPC 서버 주소
	DEFAULT_CAPTURE_INTERVAL = 1000              // 기본 캡처 주기(ms)
	DEFAULT_TARGET_FPS       = 60                // 기본 목표 FPS
	DEFAULT_FRAME_WIDTH      = 200               // 더미 프레임 폭
	DEFAULT_FRAME_HEIGHT     = 150               // 더미 프레임 높이
	DEFAULT_MONITOR_MODE     = "single"          // single | combined
	DEFAULT_MONITOR_INDEX    = 0                 // 기본 모니터 인덱스
	DEFAULT_CAPTURE_ENCODING = "png"             // png | jpeg
	DEFAULT_JPEG_QUALITY     = 80                // JPEG 품질 기본값
	DEFAULT_PREVIEW_FLAG     = false             // 기본적으로 실제 캡처는 preview 아님
)

// Config 구조체는 에이전트 실행에 필요한 환경 설정 값을 보관합니다.
type Config struct { // 단일 책임: 환경 설정 보관
	ServerAddr        string // gRPC 서버 주소
	CaptureIntervalMs int    // 캡처 주기(ms)
	TargetFPS         int    // 목표 FPS (설정 시 CaptureIntervalMs 무시)
	FrameWidth        int    // 프레임 폭 (더미 모드)
	FrameHeight       int    // 프레임 높이 (더미 모드)
	MonitorMode       string // single | combined
	MonitorIndex      int    // single 모드일 때 사용
	CaptureEncoding   string // png | jpeg
	JpegQuality       int    // jpeg 품질 (1~100)
	ForcePreview      bool   // 강제 preview 플래그
}

// Load 함수는 환경 변수에서 설정을 읽어 Config 를 반환합니다.
func Load() *Config { // 단일 책임: 환경 변수 파싱
	cfg := &Config{
		ServerAddr:        getEnvString("AGENT_SERVER_ADDR", DEFAULT_SERVER_ADDR),
		CaptureIntervalMs: getEnvInt("CAPTURE_INTERVAL_MS", DEFAULT_CAPTURE_INTERVAL),
		TargetFPS:         getEnvInt("CAPTURE_TARGET_FPS", DEFAULT_TARGET_FPS),
		FrameWidth:        getEnvInt("FRAME_WIDTH", DEFAULT_FRAME_WIDTH),
		FrameHeight:       getEnvInt("FRAME_HEIGHT", DEFAULT_FRAME_HEIGHT),
		MonitorMode:       getEnvString("CAPTURE_MONITOR_MODE", DEFAULT_MONITOR_MODE),
		MonitorIndex:      getEnvInt("CAPTURE_MONITOR_INDEX", DEFAULT_MONITOR_INDEX),
		CaptureEncoding:   getEnvString("CAPTURE_ENCODING", DEFAULT_CAPTURE_ENCODING),
		JpegQuality:       getEnvInt("JPEG_QUALITY", DEFAULT_JPEG_QUALITY),
		ForcePreview:      getEnvBool("CAPTURE_FORCE_PREVIEW", DEFAULT_PREVIEW_FLAG),
	}
	if cfg.MonitorMode != "single" && cfg.MonitorMode != "combined" { // 값 검증
		cfg.MonitorMode = DEFAULT_MONITOR_MODE
	}
	if cfg.TargetFPS < 1 || cfg.TargetFPS > 240 { // FPS 범위 검증 (1~240)
		cfg.TargetFPS = DEFAULT_TARGET_FPS
	}
	if cfg.MonitorIndex < 0 {
		cfg.MonitorIndex = 0
	}
	if cfg.CaptureEncoding != "png" && cfg.CaptureEncoding != "jpeg" {
		cfg.CaptureEncoding = DEFAULT_CAPTURE_ENCODING
	}
	if cfg.JpegQuality < 1 || cfg.JpegQuality > 100 {
		cfg.JpegQuality = DEFAULT_JPEG_QUALITY
	}
	return cfg
}

// getEnvString 함수는 문자열 환경 변수 값을 반환합니다.
func getEnvString(key, def string) string { // 단일 책임: 문자열 환경 조회
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

// getEnvInt 함수는 정수 환경 변수 값을 반환합니다.
func getEnvInt(key string, def int) int { // 단일 책임: 정수 환경 조회
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// getEnvBool 함수는 불리언 환경 변수 값을 반환합니다.
func getEnvBool(key string, def bool) bool { // 단일 책임: 불리언 환경 조회
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if v == "1" || v == "true" || v == "TRUE" || v == "True" || v == "yes" || v == "Y" {
		return true
	}
	if v == "0" || v == "false" || v == "FALSE" || v == "False" || v == "no" || v == "N" {
		return false
	}
	return def
}

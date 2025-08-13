package main

import (
	"agent/internal/agent"
	"agent/internal/config"
	"agent/internal/logging"
	"context"
)

// App struct
type App struct {
	ctx context.Context // 애플리케이션 컨텍스트

	agent *agent.Agent
}

// NewApp 함수는 App 구조체의 새 인스턴스를 생성합니다.
func NewApp() *App {
	return &App{}
}

// startup 함수는 애플리케이션 시작 시 호출되어 gRPC 연결 및 스트림을 설정합니다.
func (a *App) startup(ctx context.Context) { // 단일 책임: 앱 초기화
	baseCtx, cancel := context.WithCancel(ctx)
	a.ctx = baseCtx
	cfg := config.Load()
	logger, err := logging.NewLogger()
	if err != nil { // 실패해도 진행
		logger = nil
	}
	ag := agent.New(a.ctx, cancel, cfg, logger)
	a.agent = ag
	ag.Init()
}

// shutdown 함수는 애플리케이션 종료 시 호출되어 자원을 정리합니다.
func (a *App) shutdown(ctx context.Context) {
	a.agent.Close()
}

// StartCapture 함수는 화면 캡처를 시작합니다.
func (a *App) StartCapture() error {
	if a.agent == nil {
		return nil
	}
	return a.agent.StartCapture()
}

// StopCapture 함수는 화면 캡처를 중지합니다.
func (a *App) StopCapture() {
	if a.agent == nil {
		return
	}
	a.agent.StopCapture()
}

// ListMonitors 함수는 사용 가능한 모니터 목록 문자열 배열을 반환합니다.
func (a *App) ListMonitors() []string { // 단일 책임: 모니터 목록 노출
	if a.agent == nil {
		return nil
	}
	return a.agent.ListMonitors()
}

// SelectMonitor 함수는 단일 모드로 전환 후 특정 모니터를 선택합니다.
func (a *App) SelectMonitor(index int) bool { // 단일 책임: 모니터 선택 노출
	if a.agent == nil {
		return false
	}
	return a.agent.SelectSingleMonitor(index)
}

// SetCombinedMode 함수는 combined 모드로 전환합니다.
func (a *App) SetCombinedMode() { // 단일 책임: combined 모드 전환 노출
	if a.agent == nil {
		return
	}
	a.agent.SetCombinedMode()
}

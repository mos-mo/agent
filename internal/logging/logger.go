package logging

import (
	"go.uber.org/zap"
)

// NewLogger 함수는 환경에 맞춘 SugaredLogger 를 생성합니다.
func NewLogger() (*zap.SugaredLogger, error) { // 단일 책임: 로거 초기화
	cfg := zap.NewProductionConfig()
	l, err := cfg.Build()
	if err != nil {
		return nil, err
	}
	return l.Sugar(), nil
}

package agent

import (
	"time"

	monitorProto "agent/proto"
)

// StartCapture 함수는 주기적인 화면 캡처 루프를 시작합니다.
func (a *Agent) StartCapture() error { // 단일 책임: 캡처 루프 시작
	if a == nil || a.ctx == nil {
		return nil
	}
	if a.captureStopCh != nil { // 이미 실행 중
		return nil
	}
	a.captureStopCh = make(chan struct{})
	go a.captureLoop(a.captureStopCh)
	a.logger.Info("캡처 루프 시작")
	return nil
}

// StopCapture 함수는 캡처 루프를 중지합니다.
func (a *Agent) StopCapture() { // 단일 책임: 캡처 루프 중지
	if a.captureStopCh == nil {
		return
	}
	close(a.captureStopCh)
	a.captureStopCh = nil
	a.logger.Info("캡처 루프 중지 요청")
}

// captureLoop 함수는 설정된 주기에 따라 이미지를 캡처 후 전송합니다.
func (a *Agent) captureLoop(stopCh chan struct{}) { // 단일 책임: 캡처 반복
	// 목표 FPS 기반 프레임 간격 계산 (TargetFPS 우선, 없으면 기존 interval 사용)
	frameInterval := time.Duration(a.cfg.CaptureIntervalMs) * time.Millisecond
	if a.cfg.TargetFPS > 0 { // TargetFPS 설정 시 재계산
		frameInterval = time.Second / time.Duration(a.cfg.TargetFPS)
	}
	// 드리프트 누적 방지를 위한 nextFrameTime 사용
	nextFrameTime := time.Now()
	for {
		select {
		case <-a.ctx.Done():
			a.logger.Info("캡처 루프 컨텍스트 종료")
			return
		case <-stopCh:
			a.logger.Info("캡처 루프 종료")
			return
		default:
			// 현재 시간이 예정 시간보다 이전이면 대기
			now := time.Now()
			if wait := nextFrameTime.Sub(now); wait > 0 {
				// 짧은 sleep 으로 CPU 낭비 최소화
				time.Sleep(wait)
				continue
			}
			// 캡처 수행
			start := time.Now()
			// 캡처러 동시성 보호 (모니터 전환 중 안전성 확보)
			a.capMu.RLock()
			capt := a.capturer
			a.capMu.RUnlock()
			imgBytes, err := capt.Capture()
			if err != nil {
				a.logger.Warnf("캡처 실패: %v", err)
				// 오류 시에도 다음 프레임 시간은 고정 간격으로 진행
				nextFrameTime = nextFrameTime.Add(frameInterval)
				continue
			}
			frame := &monitorProto.FrameData{AgentId: a.agentID, ImageData: imgBytes, Timestamp: time.Now().UnixMilli(), IsPreview: a.computePreviewFlag()}
			_ = a.sendFrameData(frame)
			// 실제 처리 시간 측정 후 다음 예정 시간 계산
			nextFrameTime = nextFrameTime.Add(frameInterval)
			// 프레임 드롭 상황: 너무 뒤쳐진 경우 현재 시간으로 재조정 (버스트 방지)
			if lag := time.Since(nextFrameTime); lag > frameInterval {
				nextFrameTime = time.Now().Add(frameInterval)
			}
			// FPS 로그 (저빈도: 5초마다 1회) - 필요시 추후 개선
			_ = start // 현재는 start 변수 사용 최소화(확장 포인트)
		}
	}
}

// computePreviewFlag 함수는 프레임의 preview 여부를 계산합니다.
func (a *Agent) computePreviewFlag() bool { // 단일 책임: preview 판단
	if a.cfg.ForcePreview { // 강제 설정 우선
		return true
	}
	// 더미 캡처는 preview, 실제 캡처는 false
	if _, ok := a.capturer.(*dummyCapturer); ok {
		return true
	}
	return false
}

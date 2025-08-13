package agent

import (
	"context"
	"os"
	"runtime"
	"sync"
	"time"

	"agent/internal/config"
	monitorProto "agent/proto"

	"github.com/google/uuid"
	"go.uber.org/zap"
	grpcPkg "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	GRPC_CONNECT_MAX_ATTEMPTS  = 5                                  // gRPC 최초 연결 재시도 횟수
	GRPC_RETRY_DELAY_MS        = 1000                               // gRPC 최초 연결 재시도 지연
	INITIAL_FRAME_IS_PREVIEW   = true                               // 초기 프레임 프리뷰 여부
	INITIAL_EVENT_TYPE         = "agent_init"                       // 초기 이벤트 타입
	INITIAL_EVENT_DETAIL       = "agent started and streams opened" // 초기 이벤트 상세
	STREAM_REOPEN_MAX_ATTEMPTS = 3                                  // 스트림 재오픈 최대 시도
	STREAM_REOPEN_DELAY_MS     = 500                                // 스트림 재오픈 간격(ms)
)

type Agent struct {
	ctx         context.Context                 // 애플리케이션 컨텍스트
	cancel      context.CancelFunc              // 종료시 취소 함수
	grpcConn    *grpcPkg.ClientConn             // gRPC 연결 객체
	agentClient monitorProto.AgentServiceClient // Agent 서비스 클라이언트

	frameStream monitorProto.AgentService_StreamFramesClient // 프레임 스트림 클라이언트
	eventStream monitorProto.AgentService_StreamEventsClient // 이벤트 스트림 클라이언트

	agentID  string // 에이전트 고유 ID
	hostname string // 호스트 이름

	cfg    *config.Config     // 설정
	logger *zap.SugaredLogger // 구조화 로거
	mu     sync.Mutex         // 스트림/연결 보호

	capturer      screenCapturer // 캡처 구현
	captureStopCh chan struct{}  // 캡처 중지 채널
	capMu         sync.RWMutex   // 캡처러 교체 보호
}

func New(ctx context.Context, cancel context.CancelFunc, cfg *config.Config, logger *zap.SugaredLogger) *Agent { // 단일 책임: 에이전트 초기화
	host, _ := os.Hostname() // 호스트명 조회 (실패 시 빈 문자열)
	id := os.Getenv("AGENT_ID")
	if id == "" {
		id = uuid.New().String()
	}
	if logger == nil { // nil 안전성 확보
		l, _ := zap.NewDevelopment()
		logger = l.Sugar()
	}
	// OS 지원 시 실제 화면 캡처, 그렇지 않으면 더미
	var capt screenCapturer
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		capt = newScreenshotCapturer(cfg.MonitorMode, cfg.MonitorIndex, cfg.CaptureEncoding, cfg.JpegQuality)
	} else {
		capt = newDummyCapturer(cfg.FrameWidth, cfg.FrameHeight)
	}
	a := &Agent{
		ctx:           ctx,
		cancel:        cancel,
		agentID:       id,
		hostname:      host,
		cfg:           cfg,
		logger:        logger,
		capturer:      capt,
		captureStopCh: nil,
		capMu:         sync.RWMutex{},
	}
	return a
}

func (a *Agent) Init() { // 단일 책임: gRPC 연결 및 스트림 시작
	if err := a.connectGRPC(); err != nil {
		a.logger.Errorf("gRPC 연결 실패: %v", err)
		return
	}
	a.startStream(a.ctx)
}

func (a *Agent) startStream(ctx context.Context) { // 단일 책임: 두 개 스트림 오픈
	if err := a.openFrameStream(); err != nil {
		a.logger.Errorf("프레임 스트림 열기 실패: %v", err)
	}
	if err := a.openEventStream(); err != nil {
		a.logger.Errorf("이벤트 스트림 열기 실패: %v", err)
	}
}

func (a *Agent) connectGRPC() error { // 단일 책임: gRPC 연결 (재시도 포함)
	serverAddr := a.cfg.ServerAddr
	var lastErr error
	for attempt := 1; attempt <= GRPC_CONNECT_MAX_ATTEMPTS; attempt++ {
		conn, err := grpcPkg.DialContext(
			a.ctx,
			serverAddr,
			grpcPkg.WithTransportCredentials(insecure.NewCredentials()),
			grpcPkg.WithBlock(),
		)
		if err == nil {
			a.grpcConn = conn
			a.agentClient = monitorProto.NewAgentServiceClient(conn)
			a.logger.Infof("gRPC 연결 성공 (%s) attempt=%d", serverAddr, attempt)
			return nil
		}
		lastErr = err
		a.logger.Warnf("gRPC 연결 실패 attempt=%d err=%v", attempt, err)
		select {
		case <-time.After(time.Duration(GRPC_RETRY_DELAY_MS) * time.Millisecond):
		case <-a.ctx.Done():
			return a.ctx.Err()
		}
	}
	return lastErr
}

func (a *Agent) openFrameStream() error { // 단일 책임: 프레임 스트림 오픈
	if a.agentClient == nil {
		return nil
	}
	stream, err := a.agentClient.StreamFrames(a.ctx)
	if err != nil {
		return err
	}
	a.frameStream = stream
	a.logger.Infow("프레임 스트림 생성", "agent_id", a.agentID)
	if err := a.sendInitialFrame(); err != nil {
		a.logger.Warnf("초기 프레임 전송 실패: %v", err)
	}
	return nil
}

func (a *Agent) openEventStream() error { // 단일 책임: 이벤트 스트림 오픈
	if a.agentClient == nil {
		return nil
	}
	stream, err := a.agentClient.StreamEvents(a.ctx)
	if err != nil {
		return err
	}
	a.eventStream = stream
	a.logger.Infow("이벤트 스트림 생성", "agent_id", a.agentID)
	if err := a.sendInitialEvent(); err != nil {
		a.logger.Warnf("초기 이벤트 전송 실패: %v", err)
	}
	return nil
}

func (a *Agent) sendFrameData(frame *monitorProto.FrameData) error { // 단일 책임: 프레임 전송 + 오류 시 재시도
	a.mu.Lock()
	stream := a.frameStream
	a.mu.Unlock()
	if stream == nil {
		return nil
	}
	if err := stream.Send(frame); err != nil {
		a.logger.Warnf("프레임 전송 실패: %v - 재오픈 시도", err)
		if a.reopenFrameStream() == nil { // 성공 시 1회 재전송
			a.mu.Lock()
			if a.frameStream != nil {
				_ = a.frameStream.Send(frame)
			}
			a.mu.Unlock()
		}
		return err
	}
	return nil
}

func (a *Agent) sendEventData(event *monitorProto.EventData) error { // 단일 책임: 이벤트 전송 + 오류 시 재시도
	a.mu.Lock()
	stream := a.eventStream
	a.mu.Unlock()
	if stream == nil {
		return nil
	}
	if err := stream.Send(event); err != nil {
		a.logger.Warnf("이벤트 전송 실패: %v - 재오픈 시도", err)
		if a.reopenEventStream() == nil { // 성공 시 1회 재전송
			a.mu.Lock()
			if a.eventStream != nil {
				_ = a.eventStream.Send(event)
			}
			a.mu.Unlock()
		}
		return err
	}
	return nil
}

func (a *Agent) sendInitialFrame() error { // 단일 책임: 초기 프레임 전송
	if a.frameStream == nil {
		return nil
	}
	frame := &monitorProto.FrameData{AgentId: a.agentID, ImageData: nil, Timestamp: time.Now().UnixMilli(), IsPreview: INITIAL_FRAME_IS_PREVIEW}
	return a.frameStream.Send(frame)
}

func (a *Agent) sendInitialEvent() error { // 단일 책임: 초기 이벤트 전송
	if a.eventStream == nil {
		return nil
	}
	event := &monitorProto.EventData{AgentId: a.agentID, EventType: INITIAL_EVENT_TYPE, EventDetail: INITIAL_EVENT_DETAIL, Timestamp: time.Now().UnixMilli()}
	return a.eventStream.Send(event)
}

func (a *Agent) reopenFrameStream() error { // 단일 책임: 프레임 스트림 재오픈
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := 1; i <= STREAM_REOPEN_MAX_ATTEMPTS; i++ {
		stream, err := a.agentClient.StreamFrames(a.ctx)
		if err == nil {
			a.frameStream = stream
			a.logger.Infof("프레임 스트림 재오픈 성공 attempt=%d", i)
			if errInit := a.sendInitialFrame(); errInit != nil {
				a.logger.Warnf("재오픈 후 초기 프레임 전송 실패: %v", errInit)
			}
			return nil
		}
		a.logger.Warnf("프레임 스트림 재오픈 실패 attempt=%d err=%v", i, err)
		select {
		case <-time.After(time.Duration(STREAM_REOPEN_DELAY_MS) * time.Millisecond):
		case <-a.ctx.Done():
			return a.ctx.Err()
		}
	}
	a.frameStream = nil
	return context.Canceled
}

func (a *Agent) reopenEventStream() error { // 단일 책임: 이벤트 스트림 재오픈
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := 1; i <= STREAM_REOPEN_MAX_ATTEMPTS; i++ {
		stream, err := a.agentClient.StreamEvents(a.ctx)
		if err == nil {
			a.eventStream = stream
			a.logger.Infof("이벤트 스트림 재오픈 성공 attempt=%d", i)
			if errInit := a.sendInitialEvent(); errInit != nil {
				a.logger.Warnf("재오픈 후 초기 이벤트 전송 실패: %v", errInit)
			}
			return nil
		}
		a.logger.Warnf("이벤트 스트림 재오픈 실패 attempt=%d err=%v", i, err)
		select {
		case <-time.After(time.Duration(STREAM_REOPEN_DELAY_MS) * time.Millisecond):
		case <-a.ctx.Done():
			return a.ctx.Err()
		}
	}
	a.eventStream = nil
	return context.Canceled
}

func (a *Agent) Close() { // 단일 책임: 자원 정리
	if a.captureStopCh != nil {
		close(a.captureStopCh)
	}
	if a.frameStream != nil {
		_ = a.frameStream.CloseSend()
	}
	if a.eventStream != nil {
		_ = a.eventStream.CloseSend()
	}
	if a.grpcConn != nil {
		_ = a.grpcConn.Close()
	}
	if a.cancel != nil {
		a.cancel()
	}
	a.logger.Info("에이전트 종료 완료")
}

package agent

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"runtime"

	"github.com/kbinani/screenshot"
)

// screenCapturer 인터페이스는 화면 캡처 구현을 추상화합니다.
type screenCapturer interface { // 단일 책임: 캡처 추상화
	Capture() ([]byte, error)
}

// dummyCapturer 구조체는 더미 이미지를 생성합니다.
type dummyCapturer struct { // 단일 책임: 더미 이미지 생성
	width  int
	height int
}

// newDummyCapturer 함수는 dummyCapturer 생성자입니다.
func newDummyCapturer(width, height int) *dummyCapturer { // 단일 책임: 인스턴스 생성
	return &dummyCapturer{width: width, height: height}
}

// Capture 함수는 단색 PNG 바이트 배열을 생성합니다.
func (d *dummyCapturer) Capture() ([]byte, error) { // 단일 책임: PNG 생성
	img := image.NewRGBA(image.Rect(0, 0, d.width, d.height))
	var r, g, b uint8 = 50, 100, 150
	switch runtime.GOOS { // OS 별 색상 차등
	case "windows":
		r, g, b = 0, 120, 215
	case "darwin":
		r, g, b = 50, 50, 50
	case "linux":
		r, g, b = 60, 120, 60
	}
	for y := 0; y < d.height; y++ {
		for x := 0; x < d.width; x++ {
			offset := y*img.Stride + x*4
			img.Pix[offset+0] = r
			img.Pix[offset+1] = g
			img.Pix[offset+2] = b
			img.Pix[offset+3] = 255
		}
	}
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// screenshotCapturer 구조체는 실제 모니터 화면을 캡처합니다.
type screenshotCapturer struct { // 단일 책임: 실제 화면 캡처
	mode         string // single | combined
	monitorIndex int    // 대상 모니터 인덱스
	encoding     string // png | jpeg
	jpegQuality  int    // jpeg 품질
}

// newScreenshotCapturer 함수는 screenshotCapturer 인스턴스를 생성합니다.
func newScreenshotCapturer(mode string, idx int, encoding string, quality int) *screenshotCapturer { // 단일 책임: 인스턴스 생성
	return &screenshotCapturer{mode: mode, monitorIndex: idx, encoding: encoding, jpegQuality: quality}
}

// listMonitors 함수는 사용 가능한 모니터 개수와 각 해상도 정보를 반환합니다.
func listMonitors() []image.Rectangle { // 단일 책임: 모니터 bounds 조회
	count := screenshot.NumActiveDisplays()
	res := make([]image.Rectangle, 0, count)
	for i := 0; i < count; i++ {
		res = append(res, screenshot.GetDisplayBounds(i))
	}
	return res
}

// ListMonitors 메서드는 에이전트에서 모니터 목록을 조회(외부 노출용)합니다.
func (a *Agent) ListMonitors() []string { // 단일 책임: 모니터 정보 문자열 반환
	bounds := listMonitors()
	result := make([]string, 0, len(bounds))
	for i, b := range bounds {
		result = append(result, formatMonitorInfo(i, b))
	}
	return result
}

// formatMonitorInfo 함수는 모니터 정보를 문자열로 포맷합니다.
func formatMonitorInfo(index int, rect image.Rectangle) string { // 단일 책임: 문자열 포맷
	return fmt.Sprintf("%d:%dx%d+%d+%d", index, rect.Dx(), rect.Dy(), rect.Min.X, rect.Min.Y)
}

// SelectSingleMonitor 메서드는 single 모드로 전환 후 특정 모니터만 캡처하도록 설정합니다.
func (a *Agent) SelectSingleMonitor(index int) bool { // 단일 책임: 모니터 선택 적용
	a.capMu.Lock()
	defer a.capMu.Unlock()
	count := screenshot.NumActiveDisplays()
	if index < 0 || index >= count {
		return false
	}
	a.cfg.MonitorMode = "single"
	a.cfg.MonitorIndex = index
	a.capturer = newScreenshotCapturer("single", index, a.cfg.CaptureEncoding, a.cfg.JpegQuality)
	return true
}

// SetCombinedMode 메서드는 combined 모드로 전환합니다.
func (a *Agent) SetCombinedMode() { // 단일 책임: combined 모드 전환
	a.capMu.Lock()
	defer a.capMu.Unlock()
	a.cfg.MonitorMode = "combined"
	a.capturer = newScreenshotCapturer("combined", 0, a.cfg.CaptureEncoding, a.cfg.JpegQuality)
}

// Capture 함수는 모니터 모드에 따라 실제 화면 PNG 를 반환합니다.
func (s *screenshotCapturer) Capture() ([]byte, error) { // 단일 책임: 실제 화면 캡처
	count := screenshot.NumActiveDisplays()
	if count == 0 { // 모니터 없음
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		return s.encode(img)
	}
	if s.mode == "single" { // 단일 모니터 캡처
		if s.monitorIndex >= count {
			s.monitorIndex = 0
		}
		b := screenshot.GetDisplayBounds(s.monitorIndex)
		img, err := screenshot.CaptureRect(b)
		if err != nil {
			return nil, err
		}
		return s.encode(img)
	}
	// combined 모드: 가로로 이어붙이기
	totalWidth := 0
	maxHeight := 0
	bounds := make([]image.Rectangle, 0, count)
	for i := 0; i < count; i++ {
		b := screenshot.GetDisplayBounds(i)
		bounds = append(bounds, b)
		totalWidth += b.Dx()
		if b.Dy() > maxHeight {
			maxHeight = b.Dy()
		}
	}
	canvas := image.NewRGBA(image.Rect(0, 0, totalWidth, maxHeight))
	offsetX := 0
	for i := 0; i < count; i++ {
		b := bounds[i]
		img, err := screenshot.CaptureRect(b)
		if err != nil {
			return nil, err
		}
		target := image.Rect(offsetX, 0, offsetX+b.Dx(), b.Dy())
		draw.Draw(canvas, target, img, image.Point{}, draw.Src)
		offsetX += b.Dx()
	}
	return s.encode(canvas)
}

// encodePNG 함수는 이미지를 PNG 바이트로 인코딩합니다.
func encodePNG(img image.Image) ([]byte, error) { // 단일 책임: PNG 인코딩
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodeJPEG 함수는 이미지를 JPEG 바이트로 인코딩합니다.
func encodeJPEG(img image.Image, quality int) ([]byte, error) { // 단일 책임: JPEG 인코딩
	buf := &bytes.Buffer{}
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encode 함수는 선택한 인코딩으로 이미지를 인코딩합니다.
func (s *screenshotCapturer) encode(img image.Image) ([]byte, error) { // 단일 책임: 선택 인코딩 처리
	if s.encoding == "jpeg" {
		return encodeJPEG(img, s.jpegQuality)
	}
	return encodePNG(img)
}

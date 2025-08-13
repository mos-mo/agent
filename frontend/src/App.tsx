import { useEffect, useCallback, useState } from 'react'
import { 
  StartCapture, 
  StopCapture, 
  ListMonitors, 
  SelectMonitor, 
  SetCombinedMode 
} from "../wailsjs/go/main/App"

// 상수 정의 (대문자 스네이크 케이스)
const REFRESH_INTERVAL_MS = 5000 // 모니터 목록 자동 새로고침 주기 (ms)
const TARGET_FPS_LABEL = '30 FPS' // 고정 출력 라벨

// App 컴포넌트는 캡처 제어 및 모니터 선택 UI를 제공합니다.
const App = () => { // 단일 책임: 전체 UI 구성
  const [capturing, setCapturing] = useState(false) // 캡처 상태
  const [monitors, setMonitors] = useState<string[]>([]) // 모니터 목록
  const [selectedMonitor, setSelectedMonitor] = useState<number | null>(null) // 선택된 모니터 인덱스
  const [previousSingleMonitor, setPreviousSingleMonitor] = useState<number | null>(null) // 마지막 단일 모니터 기억
  const [mode, setMode] = useState<'single' | 'combined'>('single') // 캡처 모드
  const [message, setMessage] = useState<string>('') // 사용자 메시지
  const [loading, setLoading] = useState<boolean>(false) // 로딩 상태

  useEffect(() => {
    startCapture()
  }, [])

  // loadMonitors 함수는 모니터 목록을 불러옵니다.
  const loadMonitors = useCallback(async () => { // 단일 책임: 모니터 목록 조회
    try {
      setLoading(true)
      const list = await ListMonitors()
      setMonitors(list)
      if (list.length > 0 && selectedMonitor === null && mode === 'single') {
        setSelectedMonitor(0)
      }
    } catch (e) {
      console.error('모니터 목록 조회 실패', e)
      setMessage('모니터 목록 조회 실패')
    } finally {
      setLoading(false)
    }
  }, [selectedMonitor, mode])

  // startCapture 함수는 캡처를 시작합니다.
  const startCapture = useCallback(async () => { // 단일 책임: 캡처 시작
    if (capturing) return
    try {
      await StartCapture()
      setCapturing(true)
      setMessage('캡처 시작')
    } catch (e) {
      console.error('캡처 시작 실패', e)
      setMessage('캡처 시작 실패')
    }
  }, [capturing])

  // stopCapture 함수는 캡처를 중지합니다.
  const stopCapture = useCallback(async () => { // 단일 책임: 캡처 중지
    if (!capturing) return
    try {
      await StopCapture()
      setCapturing(false)
      setMessage('캡처 중지')
    } catch (e) {
      console.error('캡처 중지 실패', e)
      setMessage('캡처 중지 실패')
    }
  }, [capturing])

  // applySingleMonitor 함수는 단일 모드로 특정 모니터를 선택합니다.
  const applySingleMonitor = useCallback(async (index: number) => { // 단일 책임: 단일 모니터 적용
    try {
      const ok = await SelectMonitor(index)
      if (ok) {
        setMode('single')
        setSelectedMonitor(index)
  setPreviousSingleMonitor(index)
        setMessage(`모니터 ${index} 선택`)
      } else {
        setMessage('모니터 선택 실패')
      }
    } catch (e) {
      console.error('모니터 선택 실패', e)
      setMessage('모니터 선택 실패')
    }
  }, [])

  // applyCombinedMode 함수는 combined 모드로 전환합니다.
  const applyCombinedMode = useCallback(async () => { // 단일 책임: combined 모드 적용
    try {
      await SetCombinedMode()
      setMode('combined')
      setSelectedMonitor(null)
      setMessage('결합 모드 적용')
    } catch (e) {
      console.error('combined 모드 적용 실패', e)
      setMessage('결합 모드 적용 실패')
    }
  }, [])

  // switchToSingleMode 함수는 단일 모드 버튼 클릭 시 적절한 모니터로 전환합니다.
  const switchToSingleMode = useCallback(() => { // 단일 책임: 단일 모드 전환
    if (mode === 'single') return
    let target = previousSingleMonitor
    if (target === null) { // 이전 값 없으면 0 시도
      target = 0
    }
    if (monitors.length === 0) { // 모니터 없으면 메시지
      setMessage('모니터 없음 - 단일 모드 불가')
      return
    }
    if (target >= monitors.length) { // 범위 보정
      target = 0
    }
    applySingleMonitor(target)
  }, [mode, previousSingleMonitor, monitors.length, applySingleMonitor])

  // useEffect: 초기 로드 및 주기적 새로고침
  useEffect(() => { // 단일 책임: 초기 및 주기적 모니터 목록 로드
    loadMonitors()
    const id = setInterval(loadMonitors, REFRESH_INTERVAL_MS)
    return () => clearInterval(id)
  }, [loadMonitors])

  // renderMonitorList 함수는 모니터 선택 UI를 렌더링합니다.
  const renderMonitorList = () => { // 단일 책임: 모니터 리스트 렌더링
    if (mode === 'combined') {
      return <div style={{ fontSize: 13, color: '#555' }}>결합 모드 - 모든 모니터를 가로로 캡처</div>
    }
    if (monitors.length === 0) {
      return <div style={{ fontSize: 13 }}>모니터 없음</div>
    }
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        {monitors.map((m, i) => (
          <label key={i} style={{ display: 'flex', alignItems: 'center', gap: 4, cursor: 'pointer' }}>
            <input
              type="radio"
              name="monitor"
              checked={selectedMonitor === i}
              onChange={() => applySingleMonitor(i)}
            />
            <span style={{ fontSize: 13 }}>{m}</span>
          </label>
        ))}
      </div>
    )
  }

  // renderModeButtons 함수는 모드 전환 버튼을 렌더링합니다.
  const renderModeButtons = () => { // 단일 책임: 모드 전환 버튼 렌더링
    return (
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
  <button onClick={switchToSingleMode} disabled={mode === 'single'}>단일 모드</button>
        <button onClick={applyCombinedMode} disabled={mode === 'combined'}>결합 모드</button>
        <button onClick={() => loadMonitors()} disabled={loading}>목록 새로고침</button>
      </div>
    )
  }

  // renderCaptureButtons 함수는 캡처 제어 버튼을 렌더링합니다.
  const renderCaptureButtons = () => { // 단일 책임: 캡처 버튼 렌더링
    return (
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        <button onClick={startCapture} disabled={capturing}>캡처 시작</button>
        <button onClick={stopCapture} disabled={!capturing}>캡처 중지</button>
      </div>
    )
  }

  return (
    <div className="appRoot"> {/* 단일 책임: 전체 레이아웃 컨테이너 */}
      <div className="overviewPanel"> {/* 단일 책임: 오버뷰(목록/모드) 패널 */}
        <div className="panelHeader">Overview</div>
        <div className="panelGroup">
          <div className="groupTitle">모드 전환</div>
          {renderModeButtons()}
        </div>
        <div className="panelGroup largeList">
          <div className="groupTitle">모니터 선택 (single)</div>
          <div className="scrollArea">
            {renderMonitorList()}
          </div>
        </div>
        <div className="footNote">모니터: index:WxH+X+Y</div>
      </div>
      <div className="detailPanel"> {/* 단일 책임: 상세(상태/제어) 패널 */}
        <div className="panelHeader">Detail</div>
        <div className="panelGroup statusBlock">
          <div className="statusRow"><strong>캡처 상태</strong><span>{capturing ? '캡처 중' : '대기'}</span></div>
          <div className="statusRow"><strong>목표 FPS</strong><span>{TARGET_FPS_LABEL}</span></div>
          <div className="statusRow"><strong>모드</strong><span>{mode === 'combined' ? '결합' : `단일${selectedMonitor !== null ? ' #' + selectedMonitor : ''}`}</span></div>
          <div className="statusRow"><strong>모니터 수</strong><span>{monitors.length}</span></div>
        </div>
        {/*
        <div className="panelGroup">
          <div className="groupTitle">캡처 제어</div>
          {renderCaptureButtons()}
        </div>
        */}
        <div className="panelGroup messageBlock">
          {message && <div className="messageLine">알림: {message}</div>}
          {loading && <div className="loadingLine">모니터 목록 갱신 중...</div>}
        </div>
        <div className="spacer" />
        <div className="panelGroup previewPlaceholder"> {/* 단일 책임: 향후 프리뷰 자리 */}
          <div className="groupTitle">프리뷰 (향후 추가)</div>
          <div className="previewBox">프리뷰 영역 확장됨</div>
        </div>
      </div>
    </div>
  )
}

export default App
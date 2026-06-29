// PWA Sentry 섹션과 단위 테스트가 공유하는 순수 함수. DOM 접근 없음.

const HHMM_RE = /^([01]\d|2[0-3]):[0-5]\d$/;

export function isValidHHMM(s) {
  return HHMM_RE.test(s);
}

// Worker PUT 본문을 만든다. enabled는 명시적 boolean으로 정규화.
export function buildSchedulePayload({ onTime, offTime, enabled }) {
  return { onTime, offTime, enabled: !!enabled };
}

// 리스너가 실시간 상태 조회로 인식하는 ntfy 본문.
export function buildSentryStatusMessage() {
  return 'sentry status';
}

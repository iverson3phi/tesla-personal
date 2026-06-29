// 앱 UI와 단위 테스트가 공유하는 순수 함수. 여기서는 DOM에 접근하지 않는다.

// 임의 입력을 [1,3] 정수로 정규화. 숫자가 아니면 기본 3.
export function clampMinutes(raw) {
  const n = Math.round(Number(raw));
  if (!Number.isFinite(n)) return 3;
  return Math.min(3, Math.max(1, n));
}

// PC 리스너가 기대하는 ntfy 본문을 만든다.
export function buildMessage(minutes, vent) {
  const m = clampMinutes(minutes);
  return vent ? `afterblow ${m} vent` : `afterblow ${m}`;
}

// 리스너가 취소(공조off+창문닫기)로 인식하는 ntfy 본문.
export function buildCancelMessage() {
  return 'afterblow cancel';
}

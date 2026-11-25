// PWA Sentry 섹션과 단위 테스트가 공유하는 순수 함수. DOM 접근 없음.

const HHMM_RE = /^([01]\d|2[0-3]):[0-5]\d$/;

// Worker(schedule.js)의 MAX_SLOTS와 같아야 한다.
export const MAX_SLOTS = 8;

export function isValidHHMM(s) {
  return HHMM_RE.test(s);
}

// 서버 응답(구버전 onTime/offTime 단일 세트 포함)에서 슬롯 목록을 꺼낸다.
export function slotsFromSchedule(s) {
  if (s && Array.isArray(s.slots)) return s.slots.map(({ onTime, offTime }) => ({ onTime, offTime }));
  if (s && s.onTime && s.offTime) return [{ onTime: s.onTime, offTime: s.offTime }];
  return [];
}

// 저장 전 검증. 문제가 없으면 null, 있으면 사용자에게 보여줄 메시지.
export function validateSlots(slots) {
  if (!slots.length) return '시간 세트를 하나 이상 추가하세요';
  if (slots.length > MAX_SLOTS) return `시간 세트는 최대 ${MAX_SLOTS}개입니다`;
  for (const [i, s] of slots.entries()) {
    if (!isValidHHMM(s.onTime) || !isValidHHMM(s.offTime)) return `${i + 1}번 세트의 시각 형식이 올바르지 않습니다`;
    if (s.onTime === s.offTime) return `${i + 1}번 세트의 켜는 시각과 끄는 시각이 같습니다`;
  }
  return null;
}

// Worker PUT 본문을 만든다. enabled는 명시적 boolean으로 정규화.
export function buildSchedulePayload({ slots, enabled }) {
  return {
    slots: slots.map(({ onTime, offTime }) => ({ onTime, offTime })),
    enabled: !!enabled,
  };
}

// 리스너가 실시간 상태 조회로 인식하는 ntfy 본문.
export function buildSentryStatusMessage() {
  return 'sentry status';
}

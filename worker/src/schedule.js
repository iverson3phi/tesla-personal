// Sentry 스케줄 순수 로직 — I/O 없음(테스트 가능).
// 시각은 KST(UTC+9) 기준 zero-padded "HH:MM" 문자열로 다룬다.

export const DEFAULT_SCHEDULE = {
  onTime: '05:30',
  offTime: '22:16',
  enabled: true,
  lastOn: '',
  lastOff: '',
};

const pad2 = (n) => String(n).padStart(2, '0');

// epochMs(UTC) → KST 날짜/시각 조각. UTC+9를 더한 뒤 UTC 게터로 읽는다.
export function kstParts(epochMs) {
  const d = new Date(epochMs + 9 * 60 * 60 * 1000);
  const today = `${d.getUTCFullYear()}-${pad2(d.getUTCMonth() + 1)}-${pad2(d.getUTCDate())}`;
  const hhmm = `${pad2(d.getUTCHours())}:${pad2(d.getUTCMinutes())}`;
  return { today, hhmm };
}

// 발행 여부 판정. KV 갱신(부수효과)은 호출측(index.js)이 한다.
export function decideActions(state, nowParts) {
  if (!state.enabled) return { fireOn: false, fireOff: false };
  const { today, hhmm } = nowParts;
  const fireOn = hhmm >= state.onTime && state.lastOn !== today;
  const fireOff = hhmm >= state.offTime && state.lastOff !== today;
  return { fireOn, fireOff };
}

const HHMM_RE = /^([01]\d|2[0-3]):[0-5]\d$/;

// 폰이 보낸 설정 검증. lastOn/lastOff는 받지 않는다(Worker 전용).
export function validateScheduleInput(obj) {
  if (obj == null || typeof obj !== 'object') return { ok: false, error: 'body must be an object' };
  const { onTime, offTime, enabled } = obj;
  if (!HHMM_RE.test(onTime)) return { ok: false, error: 'onTime must be HH:MM' };
  if (!HHMM_RE.test(offTime)) return { ok: false, error: 'offTime must be HH:MM' };
  if (typeof enabled !== 'boolean') return { ok: false, error: 'enabled must be boolean' };
  return { ok: true, value: { onTime, offTime, enabled } };
}

// Sentry 스케줄 순수 로직 — I/O 없음(테스트 가능).
// 시각은 KST(UTC+9) 기준 zero-padded "HH:MM" 문자열로 다룬다.

// slots: ON/OFF 시각 쌍 목록(여러 세트). 각 슬롯은 [onTime, offTime) 구간이며,
// offTime < onTime 이면 자정을 넘는 구간(예: 22:00~06:00)이다.
export const MAX_SLOTS = 8;

export const DEFAULT_SCHEDULE = {
  slots: [{ onTime: '05:30', offTime: '22:16' }],
  enabled: true,
  lastState: null,
  lastStateAt: null,
  lastStateSource: null,
};

const pad2 = (n) => String(n).padStart(2, '0');

// epochMs(UTC) → KST 날짜/시각 조각. UTC+9를 더한 뒤 UTC 게터로 읽는다.
export function kstParts(epochMs) {
  const d = new Date(epochMs + 9 * 60 * 60 * 1000);
  const today = `${d.getUTCFullYear()}-${pad2(d.getUTCMonth() + 1)}-${pad2(d.getUTCDate())}`;
  const hhmm = `${pad2(d.getUTCHours())}:${pad2(d.getUTCMinutes())}`;
  return { today, hhmm };
}

const toMin = (hhmm) => Number(hhmm.slice(0, 2)) * 60 + Number(hhmm.slice(3, 5));

// 분(0~1439) t가 슬롯 구간 [on, off) 안인가. off < on 이면 자정을 넘는 구간.
function inSlot(t, slot) {
  const on = toMin(slot.onTime);
  const off = toMin(slot.offTime);
  return on <= off ? (t >= on && t < off) : (t >= on || t < off);
}

// 모든 슬롯 구간의 합집합 기준으로 분 t에 감시모드가 켜져 있어야 하는가.
export function desiredAt(slots, t) {
  return slots.some((s) => inSlot(t, s));
}

// 발행 여부 판정. 합집합이 "꺼짐→켜짐"으로 바뀌는 분에 ON, "켜짐→꺼짐"으로 바뀌는 분에 OFF.
// 슬롯이 겹쳐도(08~12 + 10~14) 12:00에 OFF 하지 않는다. 정시 판정이라 상태 가드가 없다.
export function decideActions(state, nowParts) {
  if (!state.enabled || !state.slots.length) return { fireOn: false, fireOff: false };
  const t = toMin(nowParts.hhmm);
  const now = desiredAt(state.slots, t);
  const prev = desiredAt(state.slots, (t + 1439) % 1440);
  return { fireOn: now && !prev, fireOff: !now && prev };
}

// KV에 저장된 값(구버전 onTime/offTime 단일 세트 포함)을 slots 형태로 정규화한다.
export function normalizeState(stored) {
  const { onTime, offTime, lastOn, lastOff, ...rest } = stored || {};
  const state = { ...DEFAULT_SCHEDULE, ...rest };
  if (!Array.isArray(rest.slots) && onTime && offTime) state.slots = [{ onTime, offTime }];
  return state;
}

const HHMM_RE = /^([01]\d|2[0-3]):[0-5]\d$/;

// 폰이 보낸 설정 검증. 상태 필드(lastState 등)는 받지 않는다(Worker 전용).
export function validateScheduleInput(obj) {
  if (obj == null || typeof obj !== 'object') return { ok: false, error: 'body must be an object' };
  const { slots, enabled } = obj;
  if (typeof enabled !== 'boolean') return { ok: false, error: 'enabled must be boolean' };
  if (!Array.isArray(slots) || slots.length < 1 || slots.length > MAX_SLOTS) {
    return { ok: false, error: `slots must be an array of 1..${MAX_SLOTS}` };
  }
  const clean = [];
  for (const [i, s] of slots.entries()) {
    if (s == null || typeof s !== 'object') return { ok: false, error: `slots[${i}] must be an object` };
    if (!HHMM_RE.test(s.onTime)) return { ok: false, error: `slots[${i}].onTime must be HH:MM` };
    if (!HHMM_RE.test(s.offTime)) return { ok: false, error: `slots[${i}].offTime must be HH:MM` };
    if (s.onTime === s.offTime) return { ok: false, error: `slots[${i}] onTime and offTime must differ` };
    clean.push({ onTime: s.onTime, offTime: s.offTime });
  }
  return { ok: true, value: { slots: clean, enabled } };
}

// PC가 보낸 감시모드 상태 보고 검증. lastStateAt은 Worker가 찍는다(받지 않음).
export function validateStateInput(obj) {
  if (obj == null || typeof obj !== 'object') return { ok: false, error: 'body must be an object' };
  const { state, source } = obj;
  if (state !== 'on' && state !== 'off') return { ok: false, error: 'state must be "on" or "off"' };
  if (source !== 'command' && source !== 'status') return { ok: false, error: 'source must be "command" or "status"' };
  return { ok: true, value: { state, source } };
}

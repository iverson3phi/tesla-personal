import assert from 'node:assert/strict';
import {
  DEFAULT_SCHEDULE, kstParts, decideActions, validateScheduleInput, validateStateInput,
} from './schedule.js';

// --- kstParts: UTC+9 변환 ---
// 2026-06-29T00:00:00Z = KST 2026-06-29 09:00
{
  const p = kstParts(Date.UTC(2026, 5, 29, 0, 0, 0));
  assert.equal(p.today, '2026-06-29');
  assert.equal(p.hhmm, '09:00');
}
// 2026-06-29T20:00:00Z = KST 2026-06-30 05:00 (날짜 넘어감)
{
  const p = kstParts(Date.UTC(2026, 5, 29, 20, 0, 0));
  assert.equal(p.today, '2026-06-30');
  assert.equal(p.hhmm, '05:00');
}

// --- decideActions ---
const base = { onTime: '05:30', offTime: '22:16', enabled: true, lastOn: '2026-06-28', lastOff: '2026-06-28' };
// enabled=false → 아무것도 안 함
assert.deepEqual(
  decideActions({ ...base, enabled: false }, { today: '2026-06-29', hhmm: '05:30' }),
  { fireOn: false, fireOff: false });
// onTime 도달 & 오늘 미발행 → fireOn
assert.deepEqual(
  decideActions(base, { today: '2026-06-29', hhmm: '05:30' }),
  { fireOn: true, fireOff: false });
// onTime 이전 → 발행 안 함
assert.deepEqual(
  decideActions(base, { today: '2026-06-29', hhmm: '05:29' }),
  { fireOn: false, fireOff: false });
// 오늘 이미 ON 발행됨(lastOn=today) → 재발행 안 함
assert.deepEqual(
  decideActions({ ...base, lastOn: '2026-06-29' }, { today: '2026-06-29', hhmm: '06:00' }),
  { fireOn: false, fireOff: false });
// offTime 도달 & 오늘 미발행 → fireOff (onTime은 이미 발행 가정)
assert.deepEqual(
  decideActions({ ...base, lastOn: '2026-06-29' }, { today: '2026-06-29', hhmm: '22:16' }),
  { fireOn: false, fireOff: true });

// --- validateScheduleInput ---
assert.deepEqual(
  validateScheduleInput({ onTime: '05:30', offTime: '22:16', enabled: true }),
  { ok: true, value: { onTime: '05:30', offTime: '22:16', enabled: true } });
assert.equal(validateScheduleInput({ onTime: '5:30', offTime: '22:16', enabled: true }).ok, false); // zero-pad 안 됨
assert.equal(validateScheduleInput({ onTime: '25:00', offTime: '22:16', enabled: true }).ok, false); // 시 범위 초과
assert.equal(validateScheduleInput({ onTime: '05:30', offTime: '22:16', enabled: 'yes' }).ok, false); // boolean 아님
assert.equal(validateScheduleInput({ onTime: '05:30', enabled: true }).ok, false); // offTime 누락

// --- DEFAULT_SCHEDULE 상태 필드 ---
assert.equal(DEFAULT_SCHEDULE.lastState, null);
assert.equal(DEFAULT_SCHEDULE.lastStateAt, null);
assert.equal(DEFAULT_SCHEDULE.lastStateSource, null);

// --- validateStateInput ---
assert.deepEqual(validateStateInput({ state: 'on', source: 'command' }),
  { ok: true, value: { state: 'on', source: 'command' } });
assert.deepEqual(validateStateInput({ state: 'off', source: 'status' }),
  { ok: true, value: { state: 'off', source: 'status' } });
assert.equal(validateStateInput({ state: 'maybe', source: 'command' }).ok, false);
assert.equal(validateStateInput({ state: 'on', source: 'bogus' }).ok, false);
assert.equal(validateStateInput(null).ok, false);
assert.equal(validateStateInput('x').ok, false);

console.log('schedule tests passed');

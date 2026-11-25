import assert from 'node:assert/strict';
import {
  DEFAULT_SCHEDULE, MAX_SLOTS, kstParts, decideActions, desiredAt, normalizeState,
  validateScheduleInput, validateStateInput,
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

// --- decideActions (슬롯 합집합의 경계 분에만 발행) ---
const at = (hhmm) => ({ today: '2026-06-29', hhmm });
const one = { enabled: true, slots: [{ onTime: '05:30', offTime: '22:16' }] };
assert.deepEqual(decideActions({ ...one, enabled: false }, at('05:30')), { fireOn: false, fireOff: false });
assert.deepEqual(decideActions(one, at('05:30')), { fireOn: true, fireOff: false });
assert.deepEqual(decideActions(one, at('05:29')), { fireOn: false, fireOff: false });
assert.deepEqual(decideActions(one, at('05:31')), { fireOn: false, fireOff: false }); // 정시만
assert.deepEqual(decideActions(one, at('22:16')), { fireOn: false, fireOff: true });

// 여러 세트: 05:30~08:00, 18:00~23:00
const two = { enabled: true, slots: [{ onTime: '05:30', offTime: '08:00' }, { onTime: '18:00', offTime: '23:00' }] };
assert.deepEqual(decideActions(two, at('08:00')), { fireOn: false, fireOff: true });
assert.deepEqual(decideActions(two, at('18:00')), { fireOn: true, fireOff: false });
assert.deepEqual(decideActions(two, at('23:00')), { fireOn: false, fireOff: true });
assert.deepEqual(decideActions(two, at('12:00')), { fireOn: false, fireOff: false });

// 겹치는 세트: 08~12 + 10~14 → 08 ON, 14 OFF만 (10 ON·12 OFF 없음)
const overlap = { enabled: true, slots: [{ onTime: '08:00', offTime: '12:00' }, { onTime: '10:00', offTime: '14:00' }] };
assert.deepEqual(decideActions(overlap, at('08:00')), { fireOn: true, fireOff: false });
assert.deepEqual(decideActions(overlap, at('10:00')), { fireOn: false, fireOff: false });
assert.deepEqual(decideActions(overlap, at('12:00')), { fireOn: false, fireOff: false });
assert.deepEqual(decideActions(overlap, at('14:00')), { fireOn: false, fireOff: true });

// 맞닿는 세트: 08~12 + 12~14 → 12시에 껐다 켜지 않음
const touch = { enabled: true, slots: [{ onTime: '08:00', offTime: '12:00' }, { onTime: '12:00', offTime: '14:00' }] };
assert.deepEqual(decideActions(touch, at('12:00')), { fireOn: false, fireOff: false });

// 자정을 넘는 세트: 22:00~06:00
const night = { enabled: true, slots: [{ onTime: '22:00', offTime: '06:00' }] };
assert.equal(desiredAt(night.slots, 23 * 60), true);
assert.equal(desiredAt(night.slots, 3 * 60), true);
assert.equal(desiredAt(night.slots, 12 * 60), false);
assert.deepEqual(decideActions(night, at('22:00')), { fireOn: true, fireOff: false });
assert.deepEqual(decideActions(night, at('06:00')), { fireOn: false, fireOff: true });
assert.deepEqual(decideActions(night, at('00:00')), { fireOn: false, fireOff: false });
// 00:00 ON 경계(23:59 → 00:00 전환)
assert.deepEqual(decideActions({ enabled: true, slots: [{ onTime: '00:00', offTime: '01:00' }] }, at('00:00')),
  { fireOn: true, fireOff: false });

// --- normalizeState: 구버전 단일 세트 → slots ---
assert.deepEqual(
  normalizeState({ onTime: '06:00', offTime: '21:00', enabled: false, lastOn: '2026-06-28', lastOff: '' }).slots,
  [{ onTime: '06:00', offTime: '21:00' }]);
{
  const n = normalizeState({ onTime: '06:00', offTime: '21:00', enabled: false });
  assert.equal(n.enabled, false);
  assert.equal('onTime' in n, false);
  assert.equal('lastOn' in n, false);
}
assert.deepEqual(normalizeState({ slots: [{ onTime: '01:00', offTime: '02:00' }], enabled: true }).slots,
  [{ onTime: '01:00', offTime: '02:00' }]);
assert.deepEqual(normalizeState({}).slots, DEFAULT_SCHEDULE.slots);

// --- validateScheduleInput ---
assert.deepEqual(
  validateScheduleInput({ slots: [{ onTime: '05:30', offTime: '22:16' }], enabled: true }),
  { ok: true, value: { slots: [{ onTime: '05:30', offTime: '22:16' }], enabled: true } });
// 불필요한 필드는 버린다
assert.deepEqual(
  validateScheduleInput({ slots: [{ onTime: '05:30', offTime: '22:16', x: 1 }], enabled: false }).value.slots,
  [{ onTime: '05:30', offTime: '22:16' }]);
const vs = (slots, enabled = true) => validateScheduleInput({ slots, enabled }).ok;
assert.equal(vs([{ onTime: '5:30', offTime: '22:16' }]), false);  // zero-pad 안 됨
assert.equal(vs([{ onTime: '25:00', offTime: '22:16' }]), false); // 시 범위 초과
assert.equal(vs([{ onTime: '05:30' }]), false);                   // offTime 누락
assert.equal(vs([{ onTime: '05:30', offTime: '05:30' }]), false); // ON==OFF
assert.equal(vs([]), false);                                       // 빈 목록
assert.equal(vs(Array(MAX_SLOTS + 1).fill({ onTime: '01:00', offTime: '02:00' })), false);
assert.equal(vs([null]), false);
assert.equal(vs([{ onTime: '05:30', offTime: '22:16' }], 'yes'), false); // boolean 아님
assert.equal(validateScheduleInput({ onTime: '05:30', offTime: '22:16', enabled: true }).ok, false); // 구버전 본문 거부

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

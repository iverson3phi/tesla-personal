import assert from 'node:assert/strict';
import {
  MAX_SLOTS, isValidHHMM, slotsFromSchedule, validateSlots, buildSchedulePayload, buildSentryStatusMessage,
} from './sentry.js';

assert.equal(isValidHHMM('05:30'), true);
assert.equal(isValidHHMM('23:59'), true);
assert.equal(isValidHHMM('5:30'), false);   // zero-pad 안 됨
assert.equal(isValidHHMM('24:00'), false);  // 시 범위 초과
assert.equal(isValidHHMM('05:60'), false);  // 분 범위 초과
assert.equal(isValidHHMM(''), false);

// slotsFromSchedule: 신버전 slots / 구버전 단일 세트 / 빈 값
assert.deepEqual(
  slotsFromSchedule({ slots: [{ onTime: '05:30', offTime: '08:00', x: 1 }, { onTime: '18:00', offTime: '23:00' }] }),
  [{ onTime: '05:30', offTime: '08:00' }, { onTime: '18:00', offTime: '23:00' }]);
assert.deepEqual(slotsFromSchedule({ onTime: '05:30', offTime: '22:16', enabled: true }),
  [{ onTime: '05:30', offTime: '22:16' }]);
assert.deepEqual(slotsFromSchedule(null), []);

// validateSlots
assert.equal(validateSlots([{ onTime: '05:30', offTime: '08:00' }, { onTime: '22:00', offTime: '06:00' }]), null);
assert.match(validateSlots([]), /하나 이상/);
assert.match(validateSlots(Array(MAX_SLOTS + 1).fill({ onTime: '01:00', offTime: '02:00' })), /최대/);
assert.match(validateSlots([{ onTime: '05:30', offTime: '08:00' }, { onTime: '', offTime: '09:00' }]), /^2번 세트/);
assert.match(validateSlots([{ onTime: '05:30', offTime: '05:30' }]), /같습니다/);

// buildSchedulePayload: 슬롯 필드 정리 + enabled boolean 강제
assert.deepEqual(
  buildSchedulePayload({ slots: [{ onTime: '05:30', offTime: '22:16', extra: 1 }], enabled: true }),
  { slots: [{ onTime: '05:30', offTime: '22:16' }], enabled: true });
assert.equal(buildSchedulePayload({ slots: [], enabled: 1 }).enabled, true);
assert.equal(buildSchedulePayload({ slots: [], enabled: 0 }).enabled, false);

assert.equal(buildSentryStatusMessage(), 'sentry status');

console.log('sentry tests passed');

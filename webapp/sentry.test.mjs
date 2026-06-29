import assert from 'node:assert/strict';
import { isValidHHMM, buildSchedulePayload } from './sentry.js';

assert.equal(isValidHHMM('05:30'), true);
assert.equal(isValidHHMM('23:59'), true);
assert.equal(isValidHHMM('5:30'), false);   // zero-pad 안 됨
assert.equal(isValidHHMM('24:00'), false);  // 시 범위 초과
assert.equal(isValidHHMM('05:60'), false);  // 분 범위 초과
assert.equal(isValidHHMM(''), false);

assert.deepEqual(
  buildSchedulePayload({ onTime: '05:30', offTime: '22:16', enabled: true }),
  { onTime: '05:30', offTime: '22:16', enabled: true });
// enabled를 명시적 boolean으로 강제
assert.deepEqual(
  buildSchedulePayload({ onTime: '06:00', offTime: '21:00', enabled: 1 }),
  { onTime: '06:00', offTime: '21:00', enabled: true });
assert.deepEqual(
  buildSchedulePayload({ onTime: '06:00', offTime: '21:00', enabled: 0 }),
  { onTime: '06:00', offTime: '21:00', enabled: false });

console.log('sentry tests passed');

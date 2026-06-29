import assert from 'node:assert/strict';
import { buildMessage, clampMinutes, buildCancelMessage } from './message.js';

assert.equal(clampMinutes(2), 2);
assert.equal(clampMinutes('abc'), 3);
assert.equal(clampMinutes(0), 1);
assert.equal(clampMinutes(99), 3);

assert.equal(buildMessage(2, false), 'afterblow 2');
assert.equal(buildMessage(3, true), 'afterblow 3 vent');
assert.equal(buildMessage(99, true), 'afterblow 3 vent');
assert.equal(buildMessage('abc', false), 'afterblow 3');

assert.equal(buildCancelMessage(), 'afterblow cancel');

console.log('message tests passed');

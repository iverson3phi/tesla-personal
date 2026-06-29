import assert from 'node:assert/strict';
import { buildMessage, clampMinutes } from './message.js';

assert.equal(clampMinutes(2), 2);
assert.equal(clampMinutes('abc'), 3);
assert.equal(clampMinutes(0), 1);
assert.equal(clampMinutes(99), 10);

assert.equal(buildMessage(2, false), 'afterblow 2');
assert.equal(buildMessage(3, true), 'afterblow 3 vent');
assert.equal(buildMessage(99, true), 'afterblow 10 vent');
assert.equal(buildMessage('abc', false), 'afterblow 3');

console.log('message tests passed');

import { buildMessage } from './message.js';
import { isValidHHMM, buildSchedulePayload } from './sentry.js';

const TOPIC_URL = 'https://ntfy.sh/tesla-ab-9f3k7q2zx8m';

// Task 2에서 배포한 Worker URL과 시크릿 토큰.
const SENTRY_API = 'https://tesla-sentry-scheduler.yhlee512.workers.dev/api/sentry-schedule';
const SENTRY_TOKEN = '4e9c1a7f3b2d8506e1f4a09c7d3b6e85f2a1c0d94b7e63f508a2c1d7b9e04f3a';
const SENTRY_CACHE_KEY = 'sentry-schedule';

const slider = document.getElementById('minutes');
const minutesLabel = document.getElementById('minutesLabel');
const vent = document.getElementById('vent');
const startBtn = document.getElementById('start');
const statusEl = document.getElementById('status');

function renderMinutes() {
  minutesLabel.textContent = `${slider.value}분`;
}
slider.addEventListener('input', renderMinutes);
renderMinutes();

function setStatus(text, kind) {
  statusEl.textContent = text;
  statusEl.className = `status ${kind}`;
}

async function trigger() {
  const body = buildMessage(slider.value, vent.checked);
  startBtn.disabled = true;
  setStatus('전송 중…', 'pending');
  try {
    const res = await fetch(TOPIC_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'text/plain' },
      body,
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    setStatus(`전송됨 ✓  (${body})`, 'ok');
  } catch (e) {
    setStatus(`전송 실패: ${e.message} — 다시 시도하세요`, 'err');
  } finally {
    startBtn.disabled = false;
  }
}
startBtn.addEventListener('click', trigger);

const sentryEnabled = document.getElementById('sentryEnabled');
const sentryOn = document.getElementById('sentryOn');
const sentryOff = document.getElementById('sentryOff');
const sentrySave = document.getElementById('sentrySave');
const sentryStatusEl = document.getElementById('sentryStatus');

function setSentryStatus(text, kind) {
  sentryStatusEl.textContent = text;
  sentryStatusEl.className = `status ${kind}`;
}

function applySchedule(s) {
  sentryOn.value = s.onTime;
  sentryOff.value = s.offTime;
  sentryEnabled.checked = !!s.enabled;
}

async function loadSchedule() {
  try {
    const res = await fetch(SENTRY_API);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const s = await res.json();
    applySchedule(s);
    localStorage.setItem(SENTRY_CACHE_KEY, JSON.stringify(s));
  } catch (e) {
    const cached = localStorage.getItem(SENTRY_CACHE_KEY);
    if (cached) applySchedule(JSON.parse(cached));
    setSentryStatus(`현재 설정을 못 불러왔습니다 (${e.message}) — 캐시 표시`, 'err');
  }
}

async function saveSchedule() {
  if (!isValidHHMM(sentryOn.value) || !isValidHHMM(sentryOff.value)) {
    setSentryStatus('시각 형식이 올바르지 않습니다', 'err');
    return;
  }
  const payload = buildSchedulePayload({
    onTime: sentryOn.value, offTime: sentryOff.value, enabled: sentryEnabled.checked,
  });
  sentrySave.disabled = true;
  setSentryStatus('저장 중…', 'pending');
  try {
    const res = await fetch(SENTRY_API, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${SENTRY_TOKEN}` },
      body: JSON.stringify(payload),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    localStorage.setItem(SENTRY_CACHE_KEY, JSON.stringify(payload));
    setSentryStatus('저장됨 ✓', 'ok');
  } catch (e) {
    setSentryStatus(`저장 실패: ${e.message} — 다시 시도하세요`, 'err');
  } finally {
    sentrySave.disabled = false;
  }
}

sentrySave.addEventListener('click', saveSchedule);
loadSchedule();

// 설치형/오프라인 실행을 위한 서비스워커 등록.
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('./sw.js').catch(() => {});
}

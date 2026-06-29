import { buildMessage, buildCancelMessage } from './message.js';
import { isValidHHMM, buildSchedulePayload, buildSentryStatusMessage } from './sentry.js';

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

const cancelBtn = document.getElementById('cancel');

async function triggerCancel() {
  cancelBtn.disabled = true;
  setStatus('취소 전송 중…', 'pending');
  try {
    const res = await fetch(TOPIC_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'text/plain' },
      body: buildCancelMessage(),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    setStatus('취소 전송됨 ✓ (공조 끄고 창문 닫기)', 'ok');
  } catch (e) {
    setStatus(`취소 전송 실패: ${e.message} — 다시 시도하세요`, 'err');
  } finally {
    cancelBtn.disabled = false;
  }
}
cancelBtn.addEventListener('click', triggerCancel);

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

const sentryStateEl = document.getElementById('sentryState');

function renderSentryState(s) {
  if (!sentryStateEl) return; // 현재상태 UI를 숨긴 경우 no-op (기능 코드는 유지)
  if (!s || !s.lastState) {
    sentryStateEl.textContent = '상태 미상';
    return;
  }
  const label = s.lastState === 'on' ? 'ON' : 'OFF';
  const kind = s.lastStateSource === 'status' ? '실시간' : '마지막 명령';
  sentryStateEl.textContent = `${label} (${kind})`;
}

async function loadSchedule() {
  try {
    const res = await fetch(SENTRY_API);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const s = await res.json();
    applySchedule(s);
    renderSentryState(s);
    localStorage.setItem(SENTRY_CACHE_KEY, JSON.stringify(s));
  } catch (e) {
    const cached = localStorage.getItem(SENTRY_CACHE_KEY);
    if (cached) { const c = JSON.parse(cached); applySchedule(c); renderSentryState(c); }
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

const sentryCheckBtn = document.getElementById('sentryCheck');
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function checkSentryRealtime() {
  sentryCheckBtn.disabled = true;
  // 기준 타임스탬프(이 값이 바뀌면 새 조회 결과가 들어온 것).
  let baseline = null;
  try {
    const r0 = await fetch(SENTRY_API);
    if (r0.ok) baseline = (await r0.json()).lastStateAt || null;
  } catch { /* 무시: 폴링에서 다시 시도 */ }

  setSentryStatus('실시간 확인 중… (차량을 깨우지 않음 · 오프라인이면 실패)', 'pending');
  try {
    const res = await fetch(TOPIC_URL, {
      method: 'POST', headers: { 'Content-Type': 'text/plain' }, body: buildSentryStatusMessage(),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
  } catch (e) {
    setSentryStatus(`확인 요청 실패: ${e.message}`, 'err');
    sentryCheckBtn.disabled = false;
    return;
  }

  const deadline = Date.now() + 30000;
  while (Date.now() < deadline) {
    await sleep(2000);
    try {
      const r = await fetch(SENTRY_API);
      if (!r.ok) continue;
      const s = await r.json();
      // 기준 조회가 실패했으면(baseline=null) 첫 폴링 값을 기준으로 삼아,
      // 기존에 남아 있던 status 결과를 새 결과로 오인하지 않는다.
      if (baseline === null) { baseline = s.lastStateAt || ''; continue; }
      if (s.lastStateAt && s.lastStateAt !== baseline && s.lastStateSource === 'status') {
        renderSentryState(s);
        localStorage.setItem(SENTRY_CACHE_KEY, JSON.stringify(s));
        setSentryStatus('실시간 확인 완료 ✓', 'ok');
        sentryCheckBtn.disabled = false;
        return;
      }
    } catch { /* 무시: 다음 폴링 */ }
  }
  setSentryStatus('확인 실패 — 차량이 오프라인이거나 PC가 꺼져 있을 수 있음', 'err');
  sentryCheckBtn.disabled = false;
}
// 실시간 확인 버튼은 현재 UI에서 숨김 — 버튼이 있을 때만 연결(기능 코드는 유지).
if (sentryCheckBtn) sentryCheckBtn.addEventListener('click', checkSentryRealtime);

loadSchedule();

// 설치형/오프라인 실행을 위한 서비스워커 등록.
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('./sw.js').catch(() => {});
}

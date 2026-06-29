import { buildMessage } from './message.js';

const TOPIC_URL = 'https://ntfy.sh/tesla-ab-9f3k7q2zx8m';

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

// 설치형/오프라인 실행을 위한 서비스워커 등록.
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('./sw.js').catch(() => {});
}

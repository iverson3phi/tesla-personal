import {
  DEFAULT_SCHEDULE, kstParts, decideActions, validateScheduleInput, validateStateInput,
} from './schedule.js';

const KV_KEY = 'sentry-schedule';

const CORS = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Methods': 'GET, PUT, OPTIONS',
  'Access-Control-Allow-Headers': 'Content-Type, Authorization',
};

const json = (obj, status = 200) =>
  new Response(JSON.stringify(obj), {
    status,
    headers: { 'Content-Type': 'application/json', ...CORS },
  });

async function readState(env) {
  const raw = await env.SCHEDULE_KV.get(KV_KEY);
  if (!raw) return { ...DEFAULT_SCHEDULE };
  try {
    return { ...DEFAULT_SCHEDULE, ...JSON.parse(raw) };
  } catch {
    return { ...DEFAULT_SCHEDULE };
  }
}

async function writeState(env, state) {
  await env.SCHEDULE_KV.put(KV_KEY, JSON.stringify(state));
}

async function publishNtfy(env, text) {
  // fetch는 네트워크 실패에만 reject하므로 non-2xx(429/5xx 등)도 명시적으로 던져
  // scheduled()가 lastOn/lastOff를 기록하지 않게 한다 → 그날 다음 분에 재시도.
  const res = await fetch(env.NTFY_URL, { method: 'POST', body: text });
  if (!res.ok) throw new Error(`ntfy publish failed: HTTP ${res.status}`);
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (request.method === 'OPTIONS') return new Response(null, { status: 204, headers: CORS });
    if (url.pathname === '/api/sentry-state') {
      if (request.method !== 'PUT') return json({ error: 'method not allowed' }, 405);
      const auth = request.headers.get('Authorization') || '';
      if (auth !== `Bearer ${env.SENTRY_TOKEN}`) return json({ error: 'unauthorized' }, 401);
      let body;
      try { body = await request.json(); } catch { return json({ error: 'invalid json' }, 400); }
      const v = validateStateInput(body);
      if (!v.ok) return json({ error: v.error }, 400);
      const state = await readState(env);
      const next = {
        ...state,
        lastState: v.value.state,
        lastStateSource: v.value.source,
        lastStateAt: new Date().toISOString(),
      };
      await writeState(env, next);
      return json({ ok: true, value: next });
    }
    if (url.pathname !== '/api/sentry-schedule') return new Response('not found', { status: 404, headers: CORS });

    if (request.method === 'GET') {
      return json(await readState(env));
    }

    if (request.method === 'PUT') {
      const auth = request.headers.get('Authorization') || '';
      if (auth !== `Bearer ${env.SENTRY_TOKEN}`) return json({ error: 'unauthorized' }, 401);
      let body;
      try { body = await request.json(); } catch { return json({ error: 'invalid json' }, 400); }
      const v = validateScheduleInput(body);
      if (!v.ok) return json({ error: v.error }, 400);
      const state = await readState(env);
      const next = { ...state, ...v.value, updatedAt: new Date().toISOString() };
      await writeState(env, next);
      return json({ ok: true, value: v.value });
    }

    return json({ error: 'method not allowed' }, 405);
  },

  async scheduled(event, env) {
    try {
      const state = await readState(env);
      const now = kstParts(Date.now());
      const { fireOn, fireOff } = decideActions(state, now);
      let changed = false;
      if (fireOn) {
        try {
          await publishNtfy(env, 'sentry on');
          state.lastOn = now.today; // 발행 성공 후에만 기록 → 실패 시 그날 재시도
          changed = true;
        } catch (e) {
          console.error('publish sentry on failed:', e);
        }
      }
      if (fireOff) {
        try {
          await publishNtfy(env, 'sentry off');
          state.lastOff = now.today;
          changed = true;
        } catch (e) {
          console.error('publish sentry off failed:', e);
        }
      }
      if (changed) await writeState(env, state); // 발행 성공 시에만 KV 쓰기(과금 보호)
    } catch (e) {
      console.error('scheduled run failed:', e);
    }
  },
};

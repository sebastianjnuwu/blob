type RateLimitState = {
  count: number;
  resetAt: number;
};

type RateLimitInput = {
  scope: string;
  key: string;
  max: number;
  windowMs: number;
};

export type RateLimitResult = {
  ok: boolean;
  remaining: number;
  resetAt: number;
  retryAfterMs: number;
};

const store = new Map<string, RateLimitState>();

function cleanupExpiredEntries(now: number): void {
  for (const [key, value] of store.entries()) {
    if (value.resetAt <= now) {
      store.delete(key);
    }
  }
}

/**
 * Fixed-window in-memory limiter.
 * Suitable for single-instance deployments; move to shared storage for multi-instance.
 */
export function consumeRateLimit({
  scope,
  key,
  max,
  windowMs,
}: RateLimitInput): RateLimitResult {
  const now = Date.now();
  const limitKey = `${scope}:${key}`;

  // Keep memory bounded in long-running processes.
  if (store.size > 5_000) {
    cleanupExpiredEntries(now);
  }

  const current = store.get(limitKey);

  if (!current || current.resetAt <= now) {
    const next: RateLimitState = {
      count: 1,
      resetAt: now + windowMs,
    };

    store.set(limitKey, next);

    return {
      ok: true,
      remaining: Math.max(0, max - 1),
      resetAt: next.resetAt,
      retryAfterMs: 0,
    };
  }

  if (current.count >= max) {
    return {
      ok: false,
      remaining: 0,
      resetAt: current.resetAt,
      retryAfterMs: Math.max(0, current.resetAt - now),
    };
  }

  current.count += 1;

  return {
    ok: true,
    remaining: Math.max(0, max - current.count),
    resetAt: current.resetAt,
    retryAfterMs: 0,
  };
}

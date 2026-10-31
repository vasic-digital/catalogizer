/**
 * Read a required environment variable for E2E tests.
 * There is deliberately no literal fallback: when the variable is unset or
 * empty the run fails, naming the variable (never its value).
 */
export function requireEnv(
  name: string,
  env: Record<string, string | undefined> = process.env,
): string {
  const value = env[name];
  if (value === undefined || value === '') {
    throw new Error(
      `Required environment variable ${name} is not set. ` +
        'Export it before running the E2E tests (see e2e/README.md).',
    );
  }
  return value;
}

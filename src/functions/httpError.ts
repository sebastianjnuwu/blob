export type HttpError = Error & {
  statusCode?: number;
};

/**
 * Creates a normalized HTTP-aware error object used across layers.
 */
export function createHttpError(
  message: string,
  statusCode: number,
): HttpError {
  const error = new Error(message) as HttpError;
  error.statusCode = statusCode;
  return error;
}

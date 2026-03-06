import fs from "node:fs";
import Redis from "ioredis";
import { logger } from "#functions/logger";
import "dotenv/config";

/**
 * Inicializa e conecta um cliente Redis usando ioredis.
 *
 * Configura a conexão a partir de `process.env.REDIS_URL` e TLS opcional,
 * incluindo chave e certificado. Define comportamento para reconexões
 * e verifica se o servidor está pronto antes de aceitar comandos.
 *
 * Eventos:
 * - `ready`: registra uma mensagem de debug indicando que o Redis está conectado,
 *   com a URL parcialmente mascarada.
 * - `error`: registra qualquer erro de conexão usando o logger.
 *
 * O cliente exportado pode ser usado em outras partes da aplicação
 * para operações de cache e filas.
 */

const client = new Redis(process.env.REDIS_URL, {
  maxRetriesPerRequest: null,
  enableReadyCheck: true,
  tls:
    process.env.NODE_ENV === "production" &&
    process.env.REDIS_TLS_KEY &&
    process.env.REDIS_TLS_CERT &&
    process.env.REDIS_TLS_CA
      ? {
          key: fs.readFileSync(process.env.REDIS_TLS_KEY, "utf-8"),
          cert: fs.readFileSync(process.env.REDIS_TLS_CERT, "utf-8"),
          ca: fs.readFileSync(process.env.REDIS_TLS_CA, "utf-8"),
          rejectUnauthorized: false,
        }
      : null,
});

client.on("ready", () => {
  logger.debug(
    `Redis conectado (${process.env.REDIS_URL.replace(/\/\/.*@.*/, "//***:***@***")})`,
    {
      type: "database",
    },
  );
});

client.on("error", (err) => {
  logger.error(`Redis connection error: ${err.message}`, {
    type: "database",
  });
});

export default client;

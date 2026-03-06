import fs from "node:fs/promises";
import path from "node:path";
import { PrismaBetterSqlite3 } from "@prisma/adapter-better-sqlite3";
import { PrismaClient } from "@prisma/client";
import { logger } from "#functions/logger";
import "dotenv/config";

const databaseUrl = process.env.DATABASE_URL?.startsWith("file:")
  ? process.env.DATABASE_URL
  : "file:./data/blob.db";

async function ensureSqliteDirectory() {
  if (!databaseUrl.startsWith("file:")) {
    return;
  }

  const rawPath = databaseUrl.replace("file:", "");
  const absolutePath = path.resolve(process.cwd(), rawPath);
  await fs.mkdir(path.dirname(absolutePath), { recursive: true });
}

await ensureSqliteDirectory();

const sqlitePath = path.resolve(
  process.cwd(),
  databaseUrl.replace("file:", ""),
);
const adapter = new PrismaBetterSqlite3({
  url: sqlitePath,
});

const prisma = new PrismaClient({
  adapter,
});

(async () => {
  await prisma.$queryRaw`SELECT 1`
    .then(() => {
      return logger.debug(`Prisma connected to SQLite (${databaseUrl})`, {
        type: "database",
      });
    })
    .catch((err) => {
      return logger.error(
        `Prisma not connected to the database: ${err.message}`,
        {
          type: "database",
        },
      );
    });
})();

export default prisma;

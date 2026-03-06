import pkg from "@prisma/client";

const { PrismaClient } = pkg;

import fs from "node:fs";
import { PrismaPg } from "@prisma/adapter-pg";
import { logger } from "#functions/logger";
import "dotenv/config";

const adapter = new PrismaPg({
  connectionString: process.env.DATABASE_URL,
  ssl:
    process.env.NODE_ENV === "production" &&
    process.env.DATABASE_SSL_CA &&
    process.env.DATABASE_SSL_CERT &&
    process.env.DATABASE_SSL_KEY
      ? {
          ca: fs.readFileSync(process.env.DATABASE_SSL_CA, "utf-8"),
          cert: fs.readFileSync(process.env.DATABASE_SSL_CERT, "utf-8"),
          key: fs.readFileSync(process.env.DATABASE_SSL_KEY, "utf-8"),
          rejectUnauthorized: true,
        }
      : null,
});

const prisma = new PrismaClient({
  adapter,
});

(async () => {
  await prisma.$queryRaw`SELECT 1`
    .then(() => {
      return logger.debug(
        `Prisma connected to the database (${process.env.DATABASE_URL.replace(/^(.*?:).*$/, "$1*******")})`,
        { type: "database" },
      );
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

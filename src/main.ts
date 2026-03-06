import compression from "compression";
import cors from "cors";
import express, {
  type NextFunction,
  type Request,
  type Response,
} from "express";
import helmet from "helmet";
import morgan from "morgan";
import { v7 as uuidv7 } from "uuid";
import type { HttpError } from "#functions/httpError";
import { logger } from "#functions/logger";
import blobRouter from "#routes/blob.routes";
import "dotenv/config";

const app = express();

app.set("trust proxy", 1);
app.disable("x-powered-by");
app.use(
  cors({
    origin:
      process.env.CORS_ORIGINS?.split(",")?.map((origin) => origin.trim()) ??
      "*",
    methods: ["GET", "POST", "PUT", "DELETE"],
    allowedHeaders: ["Content-Type", "Authorization"],
  }),
);
app.use(
  helmet({
    contentSecurityPolicy: {
      directives: {
        defaultSrc: ["'self'"],
        scriptSrc: ["'self'", "'unsafe-inline'"],
        styleSrc: ["'self'", "'unsafe-inline'"],
        imgSrc: ["'self'", "data:", "https:"],
        connectSrc: ["'self'"],
        fontSrc: ["'self'"],
        objectSrc: ["'none'"],
        mediaSrc: ["'self'"],
        frameSrc: ["'none'"],
      },
    },
    hsts: {
      maxAge: 31536000,
      includeSubDomains: true,
      preload: true,
    },
    referrerPolicy: {
      policy: "strict-origin-when-cross-origin",
    },
  }),
);
app.use(compression());
app.use(express.json({ limit: "250mb" }));
app.use(
  express.urlencoded({
    limit: "250mb",
    extended: true,
  }),
);

app.use((req: Request, res: Response, next: NextFunction) => {
  const requestIdHeader = req.headers["x-request-id"];
  const requestId =
    typeof requestIdHeader === "string" && requestIdHeader.length > 0
      ? requestIdHeader
      : uuidv7();

  req.requestId = requestId;
  res.setHeader("x-request-id", requestId);
  next();
});

morgan.token("id", (req: Request) => req.requestId || "unknown");

app.use(
  morgan(
    "(ID: :id) - " +
      ':remote-addr - :remote-user [:date[clf]] ":method :url HTTP/:http-version" :status :res[content-length] ":referrer" ":user-agent" (:response-time ms)',
    {
      stream: { write: (x: string) => logger.info(x.trim(), { type: "http" }) },
    },
  ),
);

app.use("/blob", blobRouter);

app.get("/", (_: Request, res: Response) => {
  res.send("OK");
});

app.get("/health", (_: Request, res: Response) => {
  res.send("OK");
});

app.use((_: Request, res: Response) => {
  res.status(404).json({ error: "Route not found" });
});

app.use(
  (error: HttpError, req: Request, res: Response, _next: NextFunction) => {
    logger.error(
      `Unhandled error (${req.requestId || "unknown"}): ${error.message}`,
    );
    res.status(error.statusCode ?? 500).json({
      error: error.message ?? "Internal server error",
      requestId: req.requestId,
    });
  },
);

const port = Number(process.env.PORT ?? 3000);
const host = process.env.HOST ?? "0.0.0.0";

app.listen(port, host, () => {
  logger.debug(`Server running: <green>http://${host}:${port}</green>`);
});

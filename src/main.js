import body from "body-parser";
import compression from "compression";
import cors from "cors";
import express from "express";
import helmet from "helmet";
import morgan from "morgan";
import { v7 as uuidv7 } from "uuid";
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
app.use(body.json({ limit: "250mb" }));
app.use(
    body.urlencoded({
        limit: "250mb",
        extended: true,
    }),
);

app.use((req, res, next) => {
    const requestId = req.headers["x-request-id"] || uuidv7();
    req.requestId = requestId;
    res.setHeader("x-request-id", requestId);
    next();
});

morgan.token("id", (req) => req.requestId);

app.use(
    morgan(
        "(ID: :id) - " +
        ':remote-addr - :remote-user [:date[clf]] ":method :url HTTP/:http-version" :status :res[content-length] ":referrer" ":user-agent" (:response-time ms)',
        {
            stream: { write: (x) => logger.info(x.trim(), { type: "http" }) },
        },
    ),
);

app.use("/blob", blobRouter);

app.get("/", (_, res) => {
    return res.send("OK");
});

app.get("/health", (_, res) => {
    return res.send("OK");
});

app.use((_, res) => {
    return res.status(404).json({ error: "Route not found" });
});

app.use((error, req, res, _) => {
    logger.error(`Unhandled error (${req.requestId}): ${error.message}`);
    return res.status(error.statusCode ?? 500).json({
        error: error.message ?? "Internal server error",
        requestId: req.requestId,
    });
});

app.listen(process.env.PORT ?? 3000, process.env.HOST ?? "0.0.0.0", () => {
    return logger.debug(
        `Server running: <green>http://${process.env.HOST ?? "0.0.0.0"}:${process.env.PORT ?? 3000}</green>`,
    );
});

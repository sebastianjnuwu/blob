import body from "body-parser";
import compression from "compression";
import cors from "cors";
import express from "express";
import helmet from "helmet";
import morgan from "morgan";
import { v7 as uuidv7 } from "uuid";
import { logger } from "#functions/logger";
import "#db/redis"
import "dotenv/config";

const app = express();

app.set("trust proxy", 1);
app.disable("x-powered-by");
app.use(
    cors({
        origin: "*",
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

app.use(
    morgan(
        `(ID: ${uuidv7()}) - ` +
        ':remote-addr - :remote-user [:date[clf]] ":method :url HTTP/:http-version" :status :res[content-length] ":referrer" ":user-agent" (:response-time ms)',
        {
            stream: { write: (x) => logger.info(x.trim(), { type: "http" }) },
        },
    ),
);

app.get("/", (_, res) => {
    return res.send("OK");
});

app.get("/health", (_, res) => {
    return res.send("OK");
});

app.listen(process.env.PORT ?? 3000, process.env.HOST ?? "0.0.0.0", () => {
    return logger.debug(
        `Server running: <green>http://${process.env.HOST ?? "0.0.0.0"}:${process.env.PORT ?? 3000}</green>`,
    );
});

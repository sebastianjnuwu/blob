import chalk from "chalk";
import { createLogger, format, transports } from "winston";
import DailyRotateFile from "winston-daily-rotate-file";

type LogLevel = "info" | "error" | "warn" | "debug";

type LogMeta = Record<string, unknown>;

type LogFunction = (message: string | Error, meta?: LogMeta) => void;

type ColorMap = Record<string, (text: string) => string>;

class AppLogger {
  private readonly logger = createLogger({
    level: "debug",
    format: format.combine(
      format.timestamp({ format: "YYYY-MM-DD HH:mm:ss" }),
      format.printf((info) => {
        let msg: string | Error = info.message as string | Error;

        if (msg instanceof Error) {
          return `Error: ${msg.message}\n${msg.stack}`;
        }

        if (typeof msg === "string") {
          const colorMap: ColorMap = {
            red: chalk.red,
            green: chalk.green,
            blue: chalk.blue,
            yellow: chalk.yellow,
            magenta: chalk.magenta,
            cyan: chalk.cyan,
            white: chalk.white,
            bold: chalk.bold,
            italic: chalk.italic,
            underline: chalk.underline,
          };
          msg = msg.replace(/<(\w+)>(.*?)<\/\1>/g, (_, color, text) => {
            const fn =
              colorMap[String(color).toLowerCase()] || ((x: string) => x);
            return fn(String(text));
          });
        }

        const prefix = chalk.cyan("[server]");
        const levelColors: Record<string, (text: string) => string> = {
          error: chalk.red,
          warn: chalk.yellow,
          info: chalk.blue,
          debug: chalk.magenta,
        };
        const levelColor = levelColors[info.level] || chalk.white;

        return `${chalk.grey(String(info.timestamp))} ${prefix} ${levelColor(info.level)}: ${msg}`;
      }),
    ),
    transports: [
      new transports.Console(),
      new DailyRotateFile({
        dirname: "./data/api/logs",
        filename: "log-%DATE%.log",
        datePattern: "YYYY-MM-DD",
        format: format.uncolorize(),
        maxFiles: "90d",
      }),
    ],
  });

  private log(level: LogLevel, message: string | Error, meta?: LogMeta): void {
    if (message instanceof Error) {
      this.logger.log({
        level,
        message: `${message.message}\n${message.stack || ""}`,
        meta,
      });
      return;
    }

    this.logger.log({ level, message, meta });
  }

  readonly info: LogFunction = (message, meta) =>
    this.log("info", message, meta);
  readonly error: LogFunction = (message, meta) =>
    this.log("error", message, meta);
  readonly warn: LogFunction = (message, meta) =>
    this.log("warn", message, meta);
  readonly debug: LogFunction = (message, meta) =>
    this.log("debug", message, meta);
}

export const logger = new AppLogger();

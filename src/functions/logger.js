import chalk from "chalk";
import { createLogger, format, transports } from "winston";
import DailyRotateFile from "winston-daily-rotate-file";

/**
 * Logger customizado para servidor.
 *
 * Fornece:
 * * Níveis de log: debug, info, warn, error
 * * Formatação de timestamp
 * * Prefixo [server] colorido
 * * Suporte a tags de cores em strings (<red>texto</red>)
 * * Log no console e arquivos rotacionados diariamente
 *
 * @class Logger
 *
 * @property {import('winston').Logger} logger Instância do logger Winston
 *
 * @method info
 * Registra uma mensagem de nível info.
 * @param {string|Error} message Mensagem ou objeto Error
 *
 * @method error
 * Registra uma mensagem de nível error.
 * @param {string|Error} message Mensagem ou objeto Error
 *
 * @method warn
 * Registra uma mensagem de nível warn.
 * @param {string|Error} message Mensagem ou objeto Error
 *
 * @method debug
 * Registra uma mensagem de nível debug.
 * @param {string|Error} message Mensagem ou objeto Error
 *
 * @constant {Logger} logger
 * Instância única do Logger para uso em toda a aplicação.
 */

class Logger {
  constructor() {
    this.logger = createLogger({
      level: "debug",
      format: format.combine(
        format.timestamp({ format: "YYYY-MM-DD HH:mm:ss" }),
        format.printf((info) => {
          let msg = info.message;

          if (msg instanceof Error)
            return `Error: ${msg.message}\n${msg.stack}`;

          if (typeof msg === "string") {
            const colorMap = {
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
            msg = msg.replace(/<(\w+)>(.*?)<\/\1>/g, (_, c, t) => {
              const fn = colorMap[c.toLowerCase()] || ((x) => x);
              return fn(t);
            });
          }

          const prefix = chalk.cyan("[server]");
          const levelColors = {
            error: chalk.red,
            warn: chalk.yellow,
            info: chalk.blue,
            debug: chalk.magenta,
          };
          const level = (levelColors[info.level] || chalk.white)(info.level);

          return `${chalk.grey(info.timestamp)} ${prefix} ${level}: ${msg}`;
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

    ["info", "error", "warn", "debug"].forEach((level) => {
      this[level] = (message, meta = undefined) =>
        this.logger.log({ level, message, meta });
    });
  }
}

export const logger = new Logger();

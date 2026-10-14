package logger

import (
	"strings"

	"github.com/fatih/color"
)

// categoryColor pinta a tag da categoria, deixando o nível com a cor dele.
const categoryColor = color.FgMagenta

// Category agrupa os métodos de log sob um prefixo fixo, ex.: [SERVER].
type Category struct {
	tag string
}

// NewCategory cria uma categoria a partir do nome, ex.: NewCategory("Cache") => [CACHE].
func NewCategory(name string) Category {
	return Category{tag: "[" + strings.ToUpper(strings.TrimSpace(name)) + "]"}
}

// log imprime [CATEGORIA] [NÍVEL] mensagem.
func (c Category) log(l string, lc color.Attribute, x string, a ...any) {
	prefix := color.New(categoryColor, color.Bold).Sprint(c.tag) + " " + color.New(lc, color.Bold).Sprint(l)
	emit(prefix, x, a...)
}

// Info loga mensagem informativa (verde).
func (c Category) Info(x string, a ...any) { c.log("[INFO]", color.FgGreen, x, a...) }

// Warn loga aviso (amarelo).
func (c Category) Warn(x string, a ...any) { c.log("[WARN]", color.FgYellow, x, a...) }

// Error loga erro (vermelho).
func (c Category) Error(x string, a ...any) { c.log("[ERROR]", color.FgRed, x, a...) }

// Debug loga mensagem de debug (ciano).
func (c Category) Debug(x string, a ...any) { c.log("[DEBUG]", color.FgCyan, x, a...) }

// Categorias disponíveis. Novas categorias: logger.NewCategory("Nome").
var (
	// Server cobre o ciclo de vida do servidor HTTP.
	Server = NewCategory("Server")
	// Database cobre conexões e operações de dados.
	Database = NewCategory("Database")
	// Environment cobre configurações e modo de execução.
	Environment = NewCategory("Environment")
	// HTTP cobre as requisições recebidas pelo servidor.
	HTTP = NewCategory("HTTP")
	// Queue cobre filas e workers (asynq).
	Queue = NewCategory("Queue")
)

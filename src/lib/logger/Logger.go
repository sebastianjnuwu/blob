package logger

import (
	"github.com/fatih/color"
	"log"
	"strconv"
	"strings"
)

// ansi monta o código ANSI a partir de um Attribute da lib color.
func ansi(a color.Attribute) string { return "\x1b[" + strconv.Itoa(int(a)) + "m" }

var m = map[string]color.Attribute{"<black>": color.FgBlack, "<red>": color.FgRed, "<green>": color.FgGreen, "<yellow>": color.FgYellow, "<blue>": color.FgBlue, "<magenta>": color.FgMagenta, "<cyan>": color.FgCyan, "<white>": color.FgWhite, "<gray>": color.FgHiBlack, "<bold>": color.Bold, "<dim>": color.Faint, "<italic>": color.Italic, "<underline>": color.Underline}
var p, s = func() (*strings.Replacer, *strings.Replacer) {
	pa, sa := []string{}, []string{}
	for k, v := range m {
		pa = append(pa, k, ansi(v))
		sa = append(sa, k, "")
		pa = append(pa, "</"+k[1:len(k)-1]+">", ansi(color.Reset))
		sa = append(sa, "</"+k[1:len(k)-1]+">", "")
	}
	pa = append(pa, "<res>", ansi(color.Reset), "<reset>", ansi(color.Reset), "</>", ansi(color.Reset))
	sa = append(sa, "<res>", "", "<reset>", "", "</>", "")
	return strings.NewReplacer(pa...), strings.NewReplacer(sa...)
}()

// paint troca as tags (<red>, </cyan>, <res>...) por ANSI ou remove se sem cor.
func paint(x string) string {
	if !strings.Contains(x, "<") {
		return x
	}
	if color.NoColor {
		return s.Replace(x)
	}
	return p.Replace(x) + ansi(color.Reset)
}

// emit imprime o prefixo colorido + mensagem com tags.
func emit(prefix, x string, a ...any) {
	log.Printf(prefix+" "+paint(x), a...)
}

// out imprime o nível colorido + mensagem com tags.
func out(l string, c color.Attribute, x string, a ...any) {
	emit(color.New(c, color.Bold).Sprint(l), x, a...)
}

// Funções sem categoria, mantidas por compatibilidade.

// Info loga mensagem informativa (verde).
func Info(x string, a ...any) { out("[INFO]", color.FgGreen, x, a...) }

// Debug loga mensagem de debug (ciano).
func Debug(x string, a ...any) { out("[DEBUG]", color.FgCyan, x, a...) }

// Warn loga aviso (amarelo).
func Warn(x string, a ...any) { out("[WARN]", color.FgYellow, x, a...) }

// Error loga erro (vermelho).
func Error(x string, a ...any) { out("[ERROR]", color.FgRed, x, a...) }

// StatusCode colore o status: 2xx verde, 3xx ciano, 4xx amarelo, 5xx vermelho.
func StatusCode(c int) string {
	v, k := strconv.Itoa(c), color.FgGreen
	if c >= 500 {
		k = color.FgRed
	} else if c >= 400 {
		k = color.FgYellow
	} else if c >= 300 {
		k = color.FgCyan
	}
	return color.New(k, color.Bold).Sprint(v)
}

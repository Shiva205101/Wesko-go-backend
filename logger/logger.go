package logger

import (
	"log/slog"
	"os"
	"strings"
)

func New(service string, env string) *slog.Logger {
	return slog.New(newHandler(env, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if attr.Key != slog.SourceKey {
				return attr
			}

			source, ok := attr.Value.Any().(*slog.Source)
			if !ok || source == nil {
				return slog.Attr{}
			}

			return slog.Group("source",
				slog.String("function", shortFunctionName(source.Function)),
				slog.Int("line", source.Line),
			)
		},
	})).With("service", service)
}

func newHandler(env string, opts *slog.HandlerOptions) slog.Handler {
	if strings.EqualFold(strings.TrimSpace(env), "prod") {
		return slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.NewTextHandler(os.Stdout, opts)
}

func shortFunctionName(name string) string {
	if name == "" {
		return ""
	}

	idx := strings.LastIndex(name, "/")
	if idx >= 0 && idx < len(name)-1 {
		name = name[idx+1:]
	}

	return name
}

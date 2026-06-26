package locale

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	Default = "zh-CN"
	English = "en"
	GinKey  = "i18n.locale"
)

type contextKey struct{}

type candidate struct {
	locale string
	q      float64
	index  int
}

func Supported(value string) bool {
	switch value {
	case Default, English:
		return true
	default:
		return false
	}
}

func Normalize(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch {
	case value == "en" || strings.HasPrefix(value, "en-"):
		return English
	case value == "zh" || strings.HasPrefix(value, "zh-"):
		return Default
	default:
		return ""
	}
}

func ResolveAcceptLanguage(header string) string {
	parts := strings.Split(header, ",")
	best := candidate{locale: Default, q: -1, index: len(parts)}
	for index, raw := range parts {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		segments := strings.Split(item, ";")
		resolved := Normalize(segments[0])
		if resolved == "" {
			continue
		}
		quality := 1.0
		for _, segment := range segments[1:] {
			keyValue := strings.SplitN(strings.TrimSpace(segment), "=", 2)
			if len(keyValue) != 2 || strings.TrimSpace(keyValue[0]) != "q" {
				continue
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(keyValue[1]), 64)
			if err != nil || parsed < 0 || parsed > 1 {
				quality = -1
				break
			}
			quality = parsed
		}
		if quality < 0 {
			continue
		}
		if quality > best.q || quality == best.q && index < best.index {
			best = candidate{locale: resolved, q: quality, index: index}
		}
	}
	if best.q < 0 {
		return Default
	}
	return best.locale
}

func WithContext(ctx context.Context, value string) context.Context {
	if !Supported(value) {
		value = Default
	}
	return context.WithValue(ctx, contextKey{}, value)
}

func FromContext(ctx context.Context) string {
	if ctx == nil {
		return Default
	}
	value, _ := ctx.Value(contextKey{}).(string)
	if Supported(value) {
		return value
	}
	return Default
}

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		resolved := ResolveAcceptLanguage(c.GetHeader("Accept-Language"))
		c.Set(GinKey, resolved)
		c.Request = c.Request.WithContext(WithContext(c.Request.Context(), resolved))
		c.Next()
	}
}

func FromGin(c *gin.Context) string {
	if c == nil {
		return Default
	}
	value, _ := c.Get(GinKey)
	if localeValue, ok := value.(string); ok && Supported(localeValue) {
		return localeValue
	}
	return FromContext(c.Request.Context())
}

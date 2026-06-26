package locale

import "testing"

func TestResolveAcceptLanguage(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{name: "empty defaults zh", header: "", want: "zh-CN"},
		{name: "exact english", header: "en", want: "en"},
		{name: "exact chinese", header: "zh-CN", want: "zh-CN"},
		{name: "generic chinese", header: "zh;q=0.9", want: "zh-CN"},
		{name: "quality prefers english", header: "zh-CN;q=0.1,en-US;q=1.0", want: "en"},
		{name: "quality prefers chinese", header: "en;q=0.2,zh-CN;q=0.8", want: "zh-CN"},
		{name: "unsupported defaults", header: "fr-FR,ja;q=0.8", want: "zh-CN"},
		{name: "bad q ignored", header: "en;q=x,zh-CN;q=0.5", want: "zh-CN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveAcceptLanguage(tt.header); got != tt.want {
				t.Fatalf("ResolveAcceptLanguage(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestContextRoundTrip(t *testing.T) {
	ctx := WithContext(t.Context(), "en")
	if got := FromContext(ctx); got != "en" {
		t.Fatalf("FromContext() = %q, want en", got)
	}
	if got := FromContext(t.Context()); got != Default {
		t.Fatalf("FromContext(empty) = %q, want %q", got, Default)
	}
}

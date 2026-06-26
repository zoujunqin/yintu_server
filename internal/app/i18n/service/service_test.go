package service

import (
	"context"
	"testing"

	"spring-slumber-server/internal/app/i18n/dao"
	"spring-slumber-server/internal/app/i18n/locale"
	"spring-slumber-server/internal/app/i18n/model"
)

type fakeStore struct {
	messages map[string]model.Message
}

func (f fakeStore) FindEnabled(_ context.Context, code, loc string) (*model.Message, error) {
	msg, ok := f.messages[code+"|"+loc]
	if !ok {
		return nil, dao.ErrMessageNotFound
	}
	return &msg, nil
}

func TestMessageUsesRequestedLocale(t *testing.T) {
	svc := NewWithStore(fakeStore{messages: map[string]model.Message{
		"invalid_body|en": {Code: "invalid_body", Locale: "en", Text: "Invalid request body"},
	}})
	ctx := locale.WithContext(t.Context(), "en")
	if got := svc.Message(ctx, "invalid_body", "fallback"); got != "Invalid request body" {
		t.Fatalf("Message() = %q", got)
	}
}

func TestMessageFallsBackToDefaultLocale(t *testing.T) {
	svc := NewWithStore(fakeStore{messages: map[string]model.Message{
		"invalid_body|zh-CN": {Code: "invalid_body", Locale: "zh-CN", Text: "请求体无效"},
	}})
	ctx := locale.WithContext(t.Context(), "en")
	if got := svc.Message(ctx, "invalid_body", "fallback"); got != "请求体无效" {
		t.Fatalf("Message() = %q", got)
	}
}

func TestMessageFallsBackToCallerFallback(t *testing.T) {
	svc := NewWithStore(fakeStore{messages: map[string]model.Message{}})
	if got := svc.Message(t.Context(), "missing", "fallback text"); got != "fallback text" {
		t.Fatalf("Message() = %q", got)
	}
}

package memory

import (
	"context"
	"sync"
	"telesrv/internal/domain"
)

// LangPackStore 是 store.LangPackStore 的内存实现。
type LangPackStore struct {
	mu sync.RWMutex
	m  map[string]domain.LangPack
}

// NewLangPackStore 创建内存 LangPackStore。
func NewLangPackStore() *LangPackStore {
	return &LangPackStore{m: make(map[string]domain.LangPack)}
}

func (s *LangPackStore) GetPack(_ context.Context, langPack, langCode string, fromVersion int) (domain.LangPack, error) {
	s.mu.RLock()
	pack := s.m[langPackKey(langPack, langCode)]
	s.mu.RUnlock()
	if pack.LangPack == "" {
		return domain.LangPack{LangPack: langPack, LangCode: langCode, FromVersion: fromVersion}, nil
	}
	pack.FromVersion = fromVersion
	if pack.Version <= fromVersion {
		pack.Strings = nil
	} else {
		pack.Strings = append([]domain.LangPackString(nil), pack.Strings...)
	}
	return pack, nil
}

func (s *LangPackStore) GetStrings(_ context.Context, langPack, langCode string, keys []string) (domain.LangPack, error) {
	s.mu.RLock()
	pack := s.m[langPackKey(langPack, langCode)]
	s.mu.RUnlock()
	if pack.LangPack == "" {
		return domain.LangPack{LangPack: langPack, LangCode: langCode}, nil
	}
	if len(keys) == 0 {
		pack.Strings = append([]domain.LangPackString(nil), pack.Strings...)
		return pack, nil
	}
	want := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		want[key] = struct{}{}
	}
	out := domain.LangPack{LangPack: pack.LangPack, LangCode: pack.LangCode, Version: pack.Version}
	for _, item := range pack.Strings {
		if _, ok := want[item.Key]; ok {
			out.Strings = append(out.Strings, item)
		}
	}
	return out, nil
}

func (s *LangPackStore) UpsertPack(_ context.Context, pack domain.LangPack) error {
	pack.Strings = append([]domain.LangPackString(nil), pack.Strings...)
	s.mu.Lock()
	s.m[langPackKey(pack.LangPack, pack.LangCode)] = pack
	s.mu.Unlock()
	return nil
}

func langPackKey(langPack, langCode string) string {
	return langPack + "\x00" + langCode
}

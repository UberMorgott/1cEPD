package settings_test

import (
	"context"
	"testing"

	"partnerops/internal/settings"
)

func TestUpdatePrefsDefaultOnAndSave(t *testing.T) {
	s, _ := openStore(t)
	ctx := context.Background()
	p, err := s.UpdatePrefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Check || !p.Install {
		t.Fatalf("по умолчанию %+v, want оба включены", p)
	}
	if err := s.SaveUpdatePrefs(ctx, settings.UpdatePrefs{Check: true, Install: false}); err != nil {
		t.Fatal(err)
	}
	if p, _ = s.UpdatePrefs(ctx); !p.Check || p.Install {
		t.Fatalf("после сохранения %+v", p)
	}
}

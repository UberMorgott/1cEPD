package settings

import (
	"context"
	"fmt"
)

// UpdatePrefs — настройки самообновления: Check — проверять новые версии
// раз в несколько часов, Install — найденную ставить самим.
type UpdatePrefs struct {
	Check   bool
	Install bool
}

// UpdatePrefs возвращает настройки самообновления.
func (s *Store) UpdatePrefs(ctx context.Context) (UpdatePrefs, error) {
	var p UpdatePrefs
	err := s.db.QueryRowContext(ctx, `SELECT update_check, update_install FROM settings WHERE id = 1`).
		Scan(&p.Check, &p.Install)
	if err != nil {
		return UpdatePrefs{}, fmt.Errorf("settings: не прочитать настройки обновления: %w", err)
	}
	return p, nil
}

// SaveUpdatePrefs перезаписывает настройки самообновления.
func (s *Store) SaveUpdatePrefs(ctx context.Context, p UpdatePrefs) error {
	_, err := s.db.ExecContext(ctx, `UPDATE settings SET update_check = ?, update_install = ? WHERE id = 1`,
		p.Check, p.Install)
	if err != nil {
		return fmt.Errorf("settings: не сохранить настройки обновления: %w", err)
	}
	return nil
}

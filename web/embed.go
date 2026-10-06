// Package web отдаёт собранный интерфейс, вшитый в бинарник.
//
// Файл лежит рядом с каталогом сборки намеренно: //go:embed не умеет подниматься
// выше своего каталога, поэтому пакет живёт в web/, а не в internal/web/.
// Иначе пришлось бы держать вторую копию dist и вручную её синхронизировать.
// Каталог dist собирается командой `npm run build` и коммитится в репозиторий:
// без него пакет не соберётся.
package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var assets embed.FS

// Handler отдаёт файлы собранного фронта.
//
// Неизвестные пути возвращают index.html: маршруты обрабатывает Vue Router
// на клиенте, и без этого перезагрузка страницы по адресу вроде /billing дала бы 404.
func Handler() (http.Handler, error) {
	root, err := fs.Sub(assets, "dist")
	if err != nil {
		return nil, fmt.Errorf("web: не найден каталог сборки: %w", err)
	}

	files := http.FileServer(http.FS(root))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}

		if _, err := fs.Stat(root, name); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		files.ServeHTTP(w, r)
	}), nil
}

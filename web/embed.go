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
//
// Исключение — файлы: путь с расширением (/assets/ClientsView-old.js) после
// обновления программы отвечает 404, а не index.html. Иначе вкладка, открытая
// до обновления, получала вместо старого куска экрана HTML, переход в меню
// молча срывался, и Клиенты и Находки показывали прежний экран. Сам index.html
// не кэшируется: перезагрузка всегда берёт имена файлов новой сборки.
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
			if path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		if name == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	}), nil
}

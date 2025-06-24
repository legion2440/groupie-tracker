# Groupie tracker

Веб-приложение Groupie Tracker отображает данные по артистам / группам из публичного API.

---

## 📝 Описание

Главная страница выводит карточки артистов / групп, детальная – полные **данные** по артисту / группе.
- данные `/artists`, `/locations`, `/dates`, `/relation` подгружаются кеш-воркером.
- названия городов приводятся к "Santiago, Chile"
- даты сортируются по времени
- Карта `locations → [dates]` формирует блок «Концерты по локациям»

**Система событий (event/action)**
- Кнопка «Обновить инфоримацию» отправляет `POST /api/refresh`; сервер асинхронно перезаполняет кеш.
**Асинхронность и кеш** 
- Фоновая горутина обновляет все данные эндпойнтов каждые 30 минут (TTL задаётся `CACHE_TTL`).
**HTTP-методы** 
- Только GET/POST; graceful-shutdown ловит `SIGINT/SIGTERM`.
**Шаблон ошибок** 
- `renderError` отдаёт `error.html` c нужным кодом.
**Unit-тесты**
- `go test ./...` проверяется парсинг `/relation` и корневой хендлер.

### Прочие "фишки"

**Cap-fallback:** 
- города преобразуются `london-uk` → `London, UK`; страны-аббревиатуры (`UK`, `USA`, `UAE`) выводятся правильным регистром.  
**Сортировка дат:** 
- звёздочка (`*`) убирается, даты парсятся и сортируются по времени.
**«Полное имя» для сольного артиста:** 
- если `members` длиной 1 – вместо списка выводится `<b>Полное имя</b>: ...`.  
**Embedded templates & static:** 
- HTML-шаблоны и JS встроены в бинарник (`//go:embed`), поэтому приложение работает из любого каталога без копирования ресурсов.  
**Кастомный TTL кеша:** 
- достаточно `export CACHE_TTL=10` - воркер будет обновляться каждые 30 минут.  

---

## ➕ Дополнительные задания

- [Filters - фильтры](filters/README.md)
- [Geolocalization - геометки](geolocalization/README.md)
- [Search bar - поисковая строка](search-bar/README.md)
- [Visualizations - визуализация](visualizations/README.md)


## 🚀 Как запустить

1. Склонировать и запустить проект
```bash
git clone https://01.tomorrow-school.ai/git/nyestaye/groupie-tracker
cd groupie-tracker
go run ./cmd/groupie-tracker
```
2. Когда проект запущен, в терминале вы увидите:
```bash
2025/06/20 17:45:21 Server started at http://localhost:8080
```
3. Перейдите на сайт http://localhost:8080


**Запуск тестов**
```bash
go test ./...
```
или
```bash
go test -v ./...
```


---

## 📁 Структура
```text
groupie-tracker/
|── filters/
│   └──  README.md            # Документация по доп. заданию Filters
|── geolocalization/
│   └──  README.md            # Документация по доп. заданию Geolocalization
|── search-bar/
│   └──  README.md            # Документация по доп. заданию Search bar
|── visualizations/
│   └──  README.md            # Документация по доп. заданию Visualizations
├── cmd/
│   └── groupie-tracker/
│       └── main.go           # Точка входа
├── internal/                 
│   ├── core/                 # обращаение к внешнему API, кеширование тестируется отдельно
│   │   ├── api_test.go       # тесты
│   │   ├── api.go
│   │   └── data.go
│   ├── model/                # plain-data структуры без методов (читается другими пакетами)
│   │   └── types.go
│   ├── server/               # HTTP-маршруты, шаблоны, graceful-shutdown
│   │   ├── templates/
│   │   │   ├── artist.html
│   │   │   ├── error.html
│   │   │   └── index.html
│   │   ├── templates_embed.go
│   │   ├── handlers_test.go
│   │   ├── handlers.go
│   │   ├── run.go
│   │   └── server.go
├── static/                   # статические JS/CSS
│   ├── *.css
│   └── refresh.js
|── go.mod                    # Go-модуль, зависимости
└── README.md                 # Документация по проекту
```

---

## 📋 TOC

- [📝 Описание](#-описание)
- [➕ Дополнительные задания](#-дополнительные-задания)
- [🚀 Как запустить](#-как-запустить)
- [📁 Структура](#-структура)

---

## 🧑‍💻 Авторы
- Anuar Turlubay
- Evelina Penkova
- Nazar Yestayev
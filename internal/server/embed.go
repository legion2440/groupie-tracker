package server

import "embed"

// Статические файлы: CSS, JS
//
//go:embed static/*
var staticFS embed.FS

// HTML-шаблоны
//
//go:embed templates/*.html
var tmplFS embed.FS

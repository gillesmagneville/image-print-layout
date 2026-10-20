// SPDX-License-Identifier: GPL-3.0-only

package i18n

var es = map[string]string{
	// Command line.
	"cli.flag.version":    "muestra la versión y termina",
	"cli.flag.selftest":   "comprueba que el motor de dibujo funciona (carga un SVG integrado, lo dibuja y lo exporta en PDF) y termina",
	"cli.flag.ready_file": "crea este archivo cuando la ventana se muestra y el documento está dibujado, y continúa (pruebas, documentación)",
	"cli.error.no_gui":    "este programa se compiló sin interfaz gráfica (cgo desactivado)",
	"cli.flag.lang":       "idioma de la interfaz: fr, en, es o de (por defecto, el del sistema)",
	"cli.warning.runtime": "Aviso: configuración del motor de dibujo: {error}",
	"cli.error.selftest":  "Fallo: {error}",
	"cli.error.language":  "idioma desconocido «{lang}»: elija fr, en, es o de",

	// Aplicación.
	"app.tagline": "Maquetación, corte e impresión de documentos vectoriales",

	// Barra de título y menú.
	"action.open_svg":         "Abrir un SVG…",
	"action.open_svg.tooltip": "Abrir un documento SVG (Ctrl+O)",
	"menu.title":              "Menú principal",
	"menu.language":           "Idioma",
	"menu.language.auto":      "Idioma del sistema",
	"menu.about":              "Acerca de",
	"menu.quit":               "Salir",

	// Panel izquierdo: ajustes.
	"panel.source.title":     "Archivo de origen",
	"panel.paper.title":      "Papel",
	"panel.margins.title":    "Márgenes",
	"panel.scale.title":      "Escala",
	"panel.placement.title":  "Posición",
	"panel.background.title": "Fondo",
	"panel.export.title":     "Exportación",
	"panel.soon":             "Disponible en una próxima versión.",
	"source.none":            "Ningún archivo abierto.",
	"source.file":            "Archivo: {name}",
	"source.size":            "Tamaño: {w} × {h} unidades",
	"source.size.mm":         "Al 100 %: {w} × {h} mm",

	// Zona central: vista previa.
	"preview.empty.title": "Ningún documento",
	"preview.empty.hint":  "Abra un documento SVG para preparar su maquetación.",

	// Panel derecho: páginas.
	"pages.title": "Páginas",
	"pages.none":  "Todavía no hay páginas.",

	// Barra de estado.
	"status.ready":   "Listo.",
	"status.loading": "Cargando {name}…",
	"status.loaded":  "{name}: {w} × {h} unidades.",

	// Cuadros de diálogo.
	"dialog.open_svg.title":    "Abrir un documento SVG",
	"dialog.filter.svg":        "Documentos SVG",
	"dialog.close":             "Cerrar",
	"dialog.error.open":        "No se pudo abrir el documento",
	"dialog.error.open.detail": "No se pudo abrir «{path}»: {reason}",
}

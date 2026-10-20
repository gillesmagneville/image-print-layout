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

	// Diagnostics: the texts of the codes of internal/diag.
	"diag.paper.unknown":          "El formato de papel «{format}» no existe.",
	"diag.paper.custom_invalid":   "Un formato personalizado necesita un ancho y un alto positivos (recibido: {w} × {h} mm).",
	"diag.paper.orientation":      "La orientación «{orientation}» no existe: elija vertical u horizontal.",
	"diag.margins.invalid":        "Los márgenes deben ser números positivos o cero.",
	"diag.margins.no_usable":      "Los márgenes no dejan espacio en la hoja (área útil: {w} × {h} mm). Redúzcalos.",
	"diag.scale.invalid_width":    "Introduzca un ancho mayor que cero.",
	"diag.scale.invalid_percent":  "Introduzca un porcentaje mayor que cero.",
	"diag.scale.invalid":          "La escala no es válida: compruebe el modo y el valor introducido.",
	"diag.doc.invalid_size":       "El documento no tiene un tamaño utilizable ({w} × {h} unidades).",
	"diag.page.too_tall":          "El contenido mide {h} mm de alto para un área útil de {usable} mm: añada un corte, o cambie la escala o el formato.",
	"diag.page.too_wide":          "El contenido mide {w} mm de ancho para un área útil de {usable} mm: cambie la escala o el formato.",
	"diag.page.shifted":           "El contenido sale del área útil por su posición: compruebe el desplazamiento o el centrado.",
	"diag.page.off_sheet":         "Una parte del contenido queda fuera de la hoja: no se puede exportar.",
	"diag.page.empty":             "Ningún bloque del dibujo está en esta página: quedará casi vacía.",
	"diag.cut.out_of_range":       "Un corte debe estar dentro del documento.",
	"diag.cut.too_close":          "Un corte debe quedar al menos a {min} mm de otro corte y del borde del documento.",
	"diag.cut.unknown":            "Este corte ya no existe.",
	"diag.cut.invalid":            "Los cortes no son válidos: deben estar en orden y dentro del documento.",
	"diag.auto.no_cut_needed":     "El dibujo cabe en una sola página: no hace falta ningún corte.",
	"diag.auto.cut_crosses":       "El corte {n} atraviesa bloques del dibujo ({count}): ningún hueco permitía evitarlos.",
	"diag.auto.cut_near":          "El corte {n} pasa a solo {mm} mm de un bloque: muévalo si queda demasiado justo.",
	"diag.auto.more_pages":        "{count} páginas en lugar de {min}: así se evita atravesar bloques.",
	"diag.correction.none":        "No hay ninguna propuesta que aplicar.",
	"diag.project.not_a_project":  "Este archivo no es un proyecto de maquetación.",
	"diag.project.too_new":        "Este proyecto lo guardó una versión más reciente del programa (formato {version}; esta versión lee hasta el formato {supported}). Actualice el programa.",
	"diag.project.bad_version":    "El número de versión del proyecto no es válido ({version}).",
	"diag.project.invalid":        "El proyecto contiene un valor que no es válido: «{field}».",
	"diag.project.source_changed": "El documento SVG ha cambiado desde que se guardó el proyecto. Los cortes están en unidades del documento: pueden dejar de caer donde los colocó.",
	"diag.project.size_changed":   "El tamaño del documento ha cambiado: {w} × {h} unidades, frente a {was_w} × {was_h} al guardar.",
	"diag.project.cuts_outside":   "Cortes que ahora quedan fuera del documento: {count}. Se quitarán si lo confirma.",
}

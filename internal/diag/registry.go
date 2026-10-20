// SPDX-License-Identifier: GPL-3.0-only

package diag

// Spec describes a code: how serious it is and the names of the values it
// carries.
type Spec struct {
	Level Level
	Args  []string
}

// Registry lists every code the program can produce. The catalogs of the
// window have a text "diag.<code>" for each, which may use the values by
// their names.
var Registry = map[string]Spec{
	// Paper and margins.
	"paper.unknown":        {Failure, []string{"format"}},
	"paper.custom_invalid": {Failure, []string{"w", "h"}},
	"paper.orientation":    {Failure, []string{"orientation"}},
	"margins.invalid":      {Failure, nil},
	"margins.no_usable":    {Failure, []string{"w", "h"}},

	// Scale.
	"scale.invalid_width":   {Failure, nil},
	"scale.invalid_percent": {Failure, nil},
	"scale.invalid":         {Failure, nil},
	"doc.invalid_size":      {Failure, []string{"w", "h"}},

	// Pages.
	"page.too_tall":  {Warning, []string{"h", "usable"}},
	"page.too_wide":  {Warning, []string{"w", "usable"}},
	"page.shifted":   {Warning, nil},
	"page.off_sheet": {Failure, nil},
	"page.empty":     {Warning, nil},

	// Cuts.
	"cut.out_of_range": {Failure, []string{"min", "max"}},
	"cut.too_close":    {Failure, []string{"min"}},
	"cut.unknown":      {Failure, nil},
	"cut.invalid":      {Failure, nil},

	// Automatic cuts.
	"auto.no_cut_needed": {Info, nil},
	"auto.cut_crosses":   {Warning, []string{"n", "count"}},
	"auto.cut_near":      {Warning, []string{"n", "mm"}},
	"auto.more_pages":    {Info, []string{"count", "min"}},

	// The document (internal/svgdoc).
	"svg.filter":          {Warning, nil},
	"svg.script":          {Warning, nil},
	"svg.animation":       {Warning, nil},
	"svg.svgfont":         {Warning, nil},
	"svg.foreign_object":  {Warning, nil},
	"svg.flow_root":       {Warning, nil},
	"svg.webfont":         {Warning, nil},
	"svg.image_missing":   {Warning, []string{"path"}},
	"svg.image_remote":    {Warning, []string{"url"}},
	"svg.images_embedded": {Info, []string{"count"}},
	"svg.use_external":    {Warning, []string{"href"}},
	"svg.no_size":         {Failure, nil},
	"svg.unreadable":      {Warning, []string{"reason"}},
	"svg.font_missing":    {Warning, []string{"requested", "used"}},
	"svg.more":            {Info, []string{"count"}},
	"svg.coarse_blocks":   {Info, []string{"count"}},

	// Export (internal/export).
	"export.no_pages":          {Failure, nil},
	"export.cuts_invalid":      {Failure, nil},
	"export.bands_sum":         {Failure, []string{"sum", "height"}},
	"export.width_differs":     {Failure, []string{"w", "want"}},
	"export.outside_sheet":     {Failure, nil},
	"export.outside_margins":   {Warning, nil},
	"export.no_background":     {Warning, nil},
	"export.background_covers": {Warning, nil},
	"export.pdf_pages":         {Failure, []string{"got", "want"}},
	"export.pdf_size":          {Failure, []string{"w", "h", "want_w", "want_h"}},
	"export.pdf_clip":          {Failure, nil},
	"export.pdf_offset":        {Failure, nil},
	"export.pdf_unreadable":    {Failure, []string{"reason"}},

	// Background.
	"background.none": {Failure, nil},

	// The application.
	"app.no_document": {Failure, nil},

	// Corrections.
	"correction.none": {Info, nil},

	// Project files.
	"project.not_a_project":   {Failure, nil},
	"project.too_new":         {Failure, []string{"version", "supported"}},
	"project.bad_version":     {Failure, []string{"version"}},
	"project.invalid":         {Failure, []string{"field"}},
	"project.source_changed":  {Warning, nil},
	"project.size_changed":    {Warning, []string{"w", "h", "was_w", "was_h"}},
	"project.cuts_outside":    {Warning, []string{"count"}},
	"project.background_lost": {Warning, nil},
}

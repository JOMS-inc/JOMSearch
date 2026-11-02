// Package static embeds the CSS and images into the binary, so the
// production image (which only contains the server binary) can serve them.
package static

import "embed"

//go:embed *.css *.png
var FS embed.FS

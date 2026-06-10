package frontend

import "embed"

// Assets contains the statically compiled Next.js frontend pages and resources
//go:embed all:out
var Assets embed.FS

// Package docs holds the user guide, which the app serves at /guide.
package docs

import "embed"

// Guide holds user-guide.md and its screenshots in guide-images/.
//
//go:embed user-guide.md guide-images
var Guide embed.FS

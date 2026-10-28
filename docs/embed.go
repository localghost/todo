// Package docs holds the user guide and the privacy policy, which the app
// serves at /guide and /privacy.
package docs

import "embed"

// Guide holds user-guide.md and its screenshots in guide-images/, and
// privacy-policy.md.
//
//go:embed user-guide.md privacy-policy.md guide-images
var Guide embed.FS

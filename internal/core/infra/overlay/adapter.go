package overlay

import (
	"context"
	"image"
	"strings"

	gridFeature "github.com/y3owk1n/neru/internal/app/components/grid"
	overlayHints "github.com/y3owk1n/neru/internal/app/components/hints"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/core/ports"
	uiOverlay "github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// Adapter implements ports.OverlayPort by wrapping the existing overlay.Manager.
type Adapter struct {
	manager uiOverlay.ManagerInterface
	logger  *zap.Logger
}

// NewAdapter creates a new overlay adapter.
func NewAdapter(manager uiOverlay.ManagerInterface, logger *zap.Logger) *Adapter {
	return &Adapter{
		manager: manager,
		logger:  logger,
	}
}

// getPrefixCharsForScreen returns the prefix characters for the current screen based on screen index.
// In multi-monitor setups, each screen gets a different set of prefix characters.
// For example, with 2 screens and 6 prefix chars per screen:
//   - Screen 0: uses chars[0:6] (A-F)
//   - Screen 1: uses chars[6:12] (G-L)
// Returns: (prefixChars, fullChars)
func getPrefixCharsForScreen(allChars string, bounds image.Rectangle) (string, string) {
	allChars = strings.ToUpper(allChars)
	if len(allChars) == 0 {
		allChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	}

	// Get screen index (0-based, left-to-right)
	screenIndex := bridge.ScreenIndexForBounds(bounds)
	if screenIndex < 0 {
		// Fallback to active screen detection
		activeBounds := bridge.ActiveScreenBounds()
		if activeBounds == bounds {
			screenIndex = 0
		} else {
			// Try to find matching screen
			allScreens := bridge.AllScreenBounds()
			for i, screen := range allScreens {
				if screen == bounds {
					screenIndex = i
					break
				}
			}
			if screenIndex < 0 {
				screenIndex = 0
			}
		}
	}

	// Calculate prefix range for this screen
	prefixCount := domainGrid.MaxPrefixCharsPerScreen
	startIndex := screenIndex * prefixCount

	// Ensure we don't exceed available characters
	if startIndex >= len(allChars) {
		// Not enough characters for this screen, fallback to first set
		startIndex = 0
	}

	endIndex := startIndex + prefixCount
	if endIndex > len(allChars) {
		endIndex = len(allChars)
	}

	return allChars[startIndex:endIndex], allChars
}

// Show shows the overlay.
func (a *Adapter) Show() {
	a.manager.Show()
}

// ShowHints displays hint labels on the screen.
func (a *Adapter) ShowHints(ctx context.Context, hints []*hint.Interface) error {
	// Check context
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	a.logger.Debug("Showing hints overlay", zap.Int("hint_count", len(hints)))

	// Convert domain hints to overlay hints for rendering
	overlayHintList := make([]*overlayHints.Hint, len(hints))
	for index, hint := range hints {
		overlayHintList[index] = overlayHints.NewHint(
			hint.Label(),
			hint.Position(),
			hint.Bounds().Size(),
			hint.MatchedPrefix(),
		)
	}

	// Show the overlay window
	a.manager.Show()
	a.manager.SwitchTo("hints")

	// Draw hints using the overlay manager
	// Retrieve config from overlay to build current style
	var style overlayHints.StyleMode
	if hintOverlay := a.manager.HintOverlay(); hintOverlay != nil {
		style = overlayHints.BuildStyle(hintOverlay.Config())
	}

	drawHintsErr := a.manager.DrawHintsWithStyle(overlayHintList, style)
	if drawHintsErr != nil {
		return derrors.Wrap(drawHintsErr, derrors.CodeOverlayFailed, "failed to draw hints")
	}

	a.logger.Info("Hints overlay displayed", zap.Int("count", len(hints)))

	return nil
}

// ShowGrid displays the grid overlay.
func (a *Adapter) ShowGrid(ctx context.Context) error {
	// Check context
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	// Get screen bounds
	bounds := bridge.ActiveScreenBounds()

	// Get prefix characters for this screen (multi-monitor support)
	prefixChars, fullChars := getPrefixCharsForScreen("abcdefghijklmnopqrstuvwxyz", bounds)

	// Create grid with screen-specific prefix characters and full character set for internal cells
	grid := domainGrid.NewGridWithPrefixChars(fullChars, prefixChars, "", "", bounds, a.logger)

	// Draw grid
	drawGridErr := a.manager.DrawGrid(grid, "", gridFeature.Style{})
	if drawGridErr != nil {
		return derrors.Wrap(drawGridErr, derrors.CodeActionFailed, "failed to draw grid")
	}

	// Show overlay and switch mode
	a.manager.Show()
	a.manager.SwitchTo("grid")

	return nil
}

// DrawScrollIndicator draws a highlight for scroll mode.
func (a *Adapter) DrawScrollIndicator(x, y int) {
	a.manager.DrawScrollIndicator(x, y)
}

// Hide removes all overlays from the screen.
func (a *Adapter) Hide(ctx context.Context) error {
	// Check context
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	a.logger.Debug("Hiding overlay")
	a.manager.Hide()
	a.manager.SwitchTo("idle")
	a.logger.Info("Overlay hidden")

	return nil
}

// IsVisible returns true if any overlay is currently visible.
func (a *Adapter) IsVisible() bool {
	return a.manager.Mode() != "idle"
}

// Refresh updates the overlay display.
func (a *Adapter) Refresh(ctx context.Context) error {
	// Check context
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	a.logger.Debug("Refreshing overlay")
	a.manager.ResizeToActiveScreen()
	a.logger.Info("Overlay refreshed")

	return nil
}

// Health checks if the overlay manager is responsive.
func (a *Adapter) Health(_ context.Context) error {
	// For now, we assume if we can call methods, it's healthy.
	// Ideally, we'd ping the UI process.
	return nil
}

// Ensure Adapter implements ports.OverlayPort.
var _ ports.OverlayPort = (*Adapter)(nil)

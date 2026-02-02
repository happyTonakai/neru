//
//  overlay.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef OVERLAY_H
#define OVERLAY_H

#import <CoreGraphics/CoreGraphics.h>
#import <Foundation/Foundation.h>

#pragma mark - Type Definitions

/// Overlay window handle
typedef void *OverlayWindow;

/// Hint style configuration
typedef struct {
	int fontSize;           ///< Font size
	char *fontFamily;       ///< Font family
	char *backgroundColor;  ///< Background color
	char *textColor;        ///< Text color
	char *matchedTextColor; ///< Matched text color
	char *borderColor;      ///< Border color
	int borderRadius;       ///< Border radius
	int borderWidth;        ///< Border width
	int padding;            ///< Padding
	double opacity;         ///< Opacity
	int showArrow;          ///< Show arrow (0 = no arrow, 1 = show arrow)
} HintStyle;

/// Hint data
typedef struct {
	char *label;             ///< Hint label
	CGPoint position;        ///< Hint position
	CGSize size;             ///< Hint size
	int matchedPrefixLength; ///< Number of matched characters to highlight
} HintData;

/// Grid cell style configuration
/// Note: Fields are ordered to ensure consistent memory layout between C and Go
typedef struct {
	// Pointer fields (8 bytes each on 64-bit) - must come first
	char *fontFamily;
	char *backgroundColor;
	char *textColor;
	char *matchedTextColor;
	char *matchedBackgroundColor;
	char *matchedBorderColor;
	char *borderColor;
	
	// Double fields (8 bytes each)
	double backgroundOpacity;
	double textOpacity;
	
	// Int fields (4 bytes each) - must come last
	int fontSize;
	int borderWidth;
} GridCellStyle;

/// Grid cell data
typedef struct {
	char *label;             ///< Cell label (display text)
	char *fullCoordinate;    ///< Full coordinate (hidden, for matching)
	CGRect bounds;           ///< Cell rectangle
	int isMatched;           ///< Cell matches current input (1 = yes, 0 = no)
	int isSubgrid;           ///< Cell is part of subgrid (1 = yes, 0 = no)
	int matchedPrefixLength; ///< Number of matched characters at beginning of label
} GridCell;

/// Region label data for 2x3 region display
typedef struct {
	char *label;    ///< Region label (A, B, C, D, E, F)
	CGRect bounds;  ///< Region rectangle
	int isMatched;  ///< Region matches current input prefix (1 = yes, 0 = no)
} RegionLabel;

/// Callback type for async operations
/// @param context Context pointer
typedef void (*ResizeCompletionCallback)(void *context);

#pragma mark - Overlay Window Functions

/// Create overlay window
/// @return Overlay window handle
OverlayWindow createOverlayWindow(void);

/// Destroy overlay window
/// @param window Overlay window handle
void NeruDestroyOverlayWindow(OverlayWindow window);

/// Show overlay window
/// @param window Overlay window handle
void NeruShowOverlayWindow(OverlayWindow window);

/// Hide overlay window
/// @param window Overlay window handle
void NeruHideOverlayWindow(OverlayWindow window);

/// Clear overlay
/// @param window Overlay window handle
void NeruClearOverlay(OverlayWindow window);

#pragma mark - Drawing Functions

/// Draw hints
/// @param window Overlay window handle
/// @param hints Array of hint data
/// @param count Number of hints
/// @param style Hint style
void NeruDrawHints(OverlayWindow window, HintData *hints, int count, HintStyle style);

/// Update hint match prefix (incremental update for typing)
/// @param window Overlay window handle
/// @param prefix Match prefix
void NeruUpdateHintMatchPrefix(OverlayWindow window, const char *prefix);

/// Draw hints incrementally (add/update/remove specific hints without clearing entire overlay)
/// @param window Overlay window handle
/// @param hintsToAdd Array of hint data to add or update
/// @param addCount Number of hints to add/update
/// @param positionsToRemove Array of hint positions to remove (by matching position)
/// @param removeCount Number of hints to remove
/// @param style Hint style (used for new/updated hints)
void NeruDrawIncrementHints(OverlayWindow window, HintData *hintsToAdd, int addCount, CGPoint *positionsToRemove,
                            int removeCount, HintStyle style);

/// Set overlay level
/// @param window Overlay window handle
/// @param level Overlay level
void NeruSetOverlayLevel(OverlayWindow window, int level);

/// Replace overlay window
/// @param pwindow Pointer to overlay window handle
void NeruReplaceOverlayWindow(OverlayWindow *pwindow);

/// Resize overlay to main screen
/// @param window Overlay window handle
void NeruResizeOverlayToMainScreen(OverlayWindow window);

/// Resize overlay to active screen
/// @param window Overlay window handle
void NeruResizeOverlayToActiveScreen(OverlayWindow window);

/// Resize overlay to active screen with callback
/// @param window Overlay window handle
/// @param callback Completion callback
/// @param context Callback context
void NeruResizeOverlayToActiveScreenWithCallback(OverlayWindow window, ResizeCompletionCallback callback,
                                                 void *context);

#pragma mark - Grid Functions

/// Draw grid cells
/// @param window Overlay window handle
/// @param cells Array of grid cells
/// @param count Number of cells
/// @param style Grid cell style
void NeruDrawGridCells(OverlayWindow window, GridCell *cells, int count, GridCellStyle style);

/// Update grid match prefix
/// @param window Overlay window handle
/// @param prefix Match prefix
void NeruUpdateGridMatchPrefix(OverlayWindow window, const char *prefix);

/// Set hide unmatched cells
/// @param window Overlay window handle
/// @param hide Hide unmatched cells (1 = yes, 0 = no)
void NeruSetHideUnmatched(OverlayWindow window, int hide);

/// Draw grid cells incrementally (add/update/remove specific cells without clearing entire overlay)
/// @param window Overlay window handle
/// @param cellsToAdd Array of grid cells to add or update
/// @param addCount Number of cells to add/update
/// @param cellsToRemove Array of cell bounds to remove (by matching bounds)
/// @param removeCount Number of cells to remove
/// @param style Grid cell style (used for new/updated cells)
void NeruDrawIncrementGrid(OverlayWindow window, GridCell *cellsToAdd, int addCount, CGRect *cellsToRemove,
                           int removeCount, GridCellStyle style);

/// Draw region labels for 2x3 region overlay
/// @param window Overlay window handle
/// @param regions Array of region labels
/// @param count Number of regions
/// @param style Grid cell style (used for font and colors)
void NeruDrawRegionLabels(OverlayWindow window, RegionLabel *regions, int count, GridCellStyle style);

/// Clear region labels
/// @param window Overlay window handle
void NeruClearRegionLabels(OverlayWindow window);

/// Set current input prefix (used to control region label visibility)
/// @param window Overlay window handle
/// @param prefix Current input prefix (empty string when no input)
void NeruSetGridInputPrefix(OverlayWindow window, const char *prefix);

#endif // OVERLAY_H

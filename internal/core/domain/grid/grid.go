package grid

import (
	"image"
	"math"
	"slices"
	"strings"
	"sync"

	"go.uber.org/zap"
)

const (
	// SmallScreenArea is the threshold for small screen area.
	SmallScreenArea = 1500000
	// MediumScreenArea is the threshold for medium screen area.
	MediumScreenArea = 2500000
	// LargeScreenArea is the threshold for large screen area.
	LargeScreenArea = 4000000

	// ExtremeAspectRatioHigh is the high threshold for extreme aspect ratios.
	ExtremeAspectRatioHigh = 2.5
	// ExtremeAspectRatioLow is the low threshold for extreme aspect ratios.
	ExtremeAspectRatioLow = 0.4
	// AspectRatioAdjustment is the adjustment factor for extreme aspect ratios.
	AspectRatioAdjustment = 1.2

	// MinCharactersLength is the minimum length for characters.
	MinCharactersLength = 2
	
	// MaxPrefixCharsPerScreen is the maximum number of characters to use as region prefixes per screen.
	// Each screen gets its own set of MaxPrefixCharsPerScreen prefix characters.
	// For example, with 2 screens: Screen 0 uses A-F, Screen 1 uses G-L.
	MaxPrefixCharsPerScreen = 6

	// MinGridCols is the minimum number of grid columns.
	MinGridCols = 2

	// MinGridRows is the minimum number of grid rows.
	MinGridRows = 2

	// MaxKeyIndex is the maximum key index.
	MaxKeyIndex = 9

	// RoundingFactor is the factor for rounding.
	RoundingFactor = 0.5

	// CenterDivisor is the divisor for center calculation.
	CenterDivisor = 2

	// ScoreWeight is the weight for scoring.
	ScoreWeight = 0.1

	// StringBuilderGrow2 is the growth for string builder.
	StringBuilderGrow2 = 2

	// StringBuilderGrow3 is the growth for string builder.
	StringBuilderGrow3 = 3

	// StringBuilderGrow4 is the growth for string builder.
	StringBuilderGrow4 = 4

	// LabelLength2 is the label length 2.
	LabelLength2 = 2

	// LabelLength3 is the label length 3.
	LabelLength3 = 3

	// LabelLength4 is the label length 4.
	LabelLength4 = 4

	// CountsCapacity is the capacity for counts.
	CountsCapacity = 5

	// PrefixLengthCheck is the check for prefix length.
	PrefixLengthCheck = 2
)

// Grid represents a coordinate grid system for spatial navigation with optimized cell sizing.
type Grid struct {
	characters string          // Characters used for coordinates (e.g., "asdfghjkl")
	rowChars   []rune          // Characters used for row labels
	colChars   []rune          // Characters used for column labels
	bounds     image.Rectangle // Screen bounds
	cells      []*Cell         // All cells with 3-char coordinates
	index      map[string]*Cell
	prefixes   map[string]bool // Set of all coordinate prefixes for fast lookup
}

// Cell represents a grid cell containing coordinate, bounds, and center point information.
type Cell struct {
	coordinate string          // 3-character coordinate (e.g., "AAA", "ABC")
	bounds     image.Rectangle // Cell bounds
	center     image.Point     // Center point
}

// Coordinate returns the 3-character coordinate.
func (c *Cell) Coordinate() string {
	return c.coordinate
}

// Bounds returns the cell bounds.
func (c *Cell) Bounds() image.Rectangle {
	return c.bounds
}

// Center returns the center point.
func (c *Cell) Center() image.Point {
	return c.center
}

// NewGrid creates a grid with automatically optimized cell sizes for the screen.
// Cell sizes are dynamically calculated based on screen dimensions, resolution, and aspect ratio
// to ensure optimal precision and usability across all display types.
//
// Grid layout uses spatial regions for predictable navigation:
//   - Each region is identified by the first character (Region A, Region B, etc.)
//   - Within each region, coordinates flow left-to-right, top-to-bottom
//   - Region A: AAA, ABA, ACA (left-to-right), then AAB, ABB, ACB (next row)
//   - Regions flow left-to-right until screen width is filled
//   - Next region starts on new row below, continuing the pattern
//   - This allows users to think: "C** coordinates are in region C on the screen"
//
// Cell sizing is fully automatic based on screen characteristics:
//   - Very small screens (<1.5M pixels): 25-60px cells for maximum precision
//   - Small-medium screens (1.5-2.5M pixels): 30-80px cells
//   - Medium-large screens (2.5-4M pixels): 40-100px cells
//   - Very large screens (>4M pixels): 50-120px cells
//
// Multi-monitor support: Each screen gets its own set of prefix characters.
// For example, with characters="ABCDEFGHIJKLMNOPQRSTUVWXYZ":
//   - Screen 0 (left): uses A-F as prefixes
//   - Screen 1 (right): uses G-L as prefixes
//
// If rowLabels or colLabels are empty, they will be inferred from characters.
func NewGrid(characters string, bounds image.Rectangle, logger *zap.Logger) *Grid {
	return NewGridWithPrefixChars(characters, characters, "", "", bounds, logger)
}

// NewGridWithLabels creates a grid with custom row and column labels.
// If rowLabels or colLabels are empty, they will be inferred from characters.
func NewGridWithLabels(
	characters, rowLabels, colLabels string,
	bounds image.Rectangle,
	logger *zap.Logger,
) *Grid {
	return NewGridWithPrefixChars(characters, "", rowLabels, colLabels, bounds, logger)
}

// NewGridWithPrefixChars creates a grid with separate prefix characters for screen regions.
//   - prefixChars: characters used for region prefixes (first letter of coordinate)
//   - characters: characters used for internal grid cells (second and third letters)
//   - If prefixChars is empty, it will be inferred from characters
func NewGridWithPrefixChars(
	characters, prefixChars, rowLabels, colLabels string,
	bounds image.Rectangle,
	logger *zap.Logger,
) *Grid {
	logger.Debug("Creating new grid",
		zap.String("characters", characters),
		zap.String("prefixChars", prefixChars),
		zap.String("rowLabels", rowLabels),
		zap.String("colLabels", colLabels),
		zap.Int("bounds_width", bounds.Dx()),
		zap.Int("bounds_height", bounds.Dy()))

	if characters == "" {
		characters = "abcdefghijklmnopqrstuvwxyz"
	}

	// Use prefixChars if provided, otherwise use first part of characters
	uppercasePrefixChars := strings.ToUpper(prefixChars)
	if uppercasePrefixChars == "" {
		uppercasePrefixChars = strings.ToUpper(characters)
	}

	// For prefix chars, limit to MaxPrefixCharsPerScreen
	prefixRunes := []rune(uppercasePrefixChars)
	if len(prefixRunes) > MaxPrefixCharsPerScreen {
		prefixRunes = prefixRunes[:MaxPrefixCharsPerScreen]
		uppercasePrefixChars = string(prefixRunes)
	}

	// Cache uppercase conversion once at the start
	uppercaseChars := strings.ToUpper(characters)
	chars := []rune(uppercaseChars)
	numChars := len(chars)

	// Ensure we have valid characters
	if numChars < MinCharactersLength {
		uppercaseChars = strings.ToUpper("abcdefghijklmnopqrstuvwxyz")
		chars = []rune(uppercaseChars)
		numChars = len(chars)
	}

	// Use prefix chars for region identification
	// Use full chars for internal grid cells
	prefixChars = uppercasePrefixChars
	prefixNumChars := len(prefixRunes)

	// Prepare row and column labels (always use full character set)
	rowChars := chars
	colChars := chars

	if rowLabels != "" {
		rowChars = []rune(strings.ToUpper(rowLabels))
	}

	if colLabels != "" {
		colChars = []rune(strings.ToUpper(colLabels))
	}

	numRowChars := len(rowChars)
	numColChars := len(colChars)

	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	logger.Debug("Grid dimensions calculated",
		zap.Int("width", width),
		zap.Int("height", height))

	if gridCacheEnabled {
		if cells, ok := gridCache.get(uppercaseChars, strings.ToUpper(rowLabels), strings.ToUpper(colLabels), bounds); ok {
			logger.Debug("Grid cache hit",
				zap.Int("cell_count", len(cells)))

			// Pre-allocate index map with exact capacity
			index := make(map[string]*Cell, len(cells))
			for _, cell := range cells {
				index[cell.Coordinate()] = cell
			}

			// Build prefix index for fast prefix matching
			prefixes := buildPrefixIndex(cells)

			return &Grid{
				characters: uppercaseChars,
				rowChars:   rowChars,
				colChars:   colChars,
				bounds:     bounds,
				cells:      cells,
				index:      index,
				prefixes:   prefixes,
			}
		}

		logger.Debug("Grid cache miss")
	}

	if width <= 0 || height <= 0 {
		logger.Warn("Invalid grid bounds, creating minimal grid",
			zap.Int("width", width),
			zap.Int("height", height))

		return &Grid{
			characters: uppercaseChars,
			bounds:     bounds,
			cells:      []*Cell{},
		}
	}

	// Automatically determine optimal cell size constraints based on screen characteristics
	minCellSize, maxCellSize := calculateOptimalCellSizes(width, height)

	// Find all valid grid configurations and pick the one with best aspect ratio match
	// This ensures cells are as square as possible for intuitive navigation
	candidates := findValidGridConfigurations(width, height, minCellSize, maxCellSize)

	// Pick the candidate with the best (lowest) score
	gridCols, gridRows := selectBestCandidate(candidates, width, height, minCellSize, maxCellSize)

	// Safety check: ensure we always have at least a 2x2 grid
	if gridCols < MinGridCols {
		gridCols = 2
	}

	if gridRows < MinGridRows {
		gridRows = 2
	}

	// Calculate total cells needed to fill screen
	totalCells := gridRows * gridCols

	// Determine optimal label length based on total cells and available characters
	labelLength := calculateLabelLength(totalCells, numChars, numRowChars, numColChars)

	// Calculate maximum possible cells we can label based on label length
	var maxPossibleCells int
	switch labelLength {
	case LabelLength2:
		maxPossibleCells = numChars * numColChars
	case LabelLength3:
		maxPossibleCells = numChars * numColChars * numRowChars
	default:
		maxPossibleCells = numChars * numChars * numColChars * numRowChars
	}

	// Cap totalCells to what we can actually label
	if totalCells > maxPossibleCells {
		// Calculate grid dimensions that fit within maxPossibleCells
		gridCols = gridMax(
			int(math.Sqrt(float64(maxPossibleCells)*float64(width)/float64(height))),
			1,
		)
		gridRows = gridMax(maxPossibleCells/gridCols, 1)
		// Update totalCells to match the actual grid dimensions
		totalCells = gridRows * gridCols //nolint:ineffassign,staticcheck,wastedassign // totalCells is used later in calculateLabelLength
	}

	// Calculate base cell sizes and remainders
	baseCellWidth := width / gridCols
	baseCellHeight := height / gridRows
	remainderWidth := width % gridCols
	remainderHeight := height % gridRows

	// Generate cells with spatial region logic
	cells := generateCellsWithRegions(
		prefixChars,
		prefixNumChars,
		chars,
		rowChars,
		colChars,
		numChars,
		gridCols,
		gridRows,
		labelLength,
		bounds,
		baseCellWidth,
		baseCellHeight,
		remainderWidth,
		remainderHeight,
		logger,
	)

	logger.Debug("Grid created successfully",
		zap.Int("cell_count", len(cells)),
		zap.Int("grid_cols", gridCols),
		zap.Int("grid_rows", gridRows),
		zap.Int("label_length", labelLength))

	if gridCacheEnabled {
		gridCache.put(
			uppercaseChars,
			strings.ToUpper(rowLabels),
			strings.ToUpper(colLabels),
			bounds,
			cells,
		)
		logger.Debug("Grid cache store",
			zap.Int("cell_count", len(cells)))
	}

	// Pre-allocate index map with exact capacity
	index := make(map[string]*Cell, len(cells))
	for _, cell := range cells {
		index[cell.Coordinate()] = cell
	}

	// Build prefix index for fast prefix matching
	prefixes := buildPrefixIndex(cells)

	return &Grid{
		characters: uppercaseChars,
		rowChars:   rowChars,
		colChars:   colChars,
		bounds:     bounds,
		cells:      cells,
		index:      index,
		prefixes:   prefixes,
	}
}

// Characters returns the characters used for coordinates.
func (g *Grid) Characters() string {
	return g.characters
}

// RowLabels returns the row labels used for coordinates.
func (g *Grid) RowLabels() string {
	return string(g.rowChars)
}

// ColLabels returns the column labels used for coordinates.
func (g *Grid) ColLabels() string {
	return string(g.colChars)
}

// ValidCharacters returns all characters that can appear in grid coordinates.
func (g *Grid) ValidCharacters() string {
	// If no custom labels, return the main characters
	if len(g.rowChars) == 0 && len(g.colChars) == 0 {
		return g.characters
	}

	charSet := make(map[rune]bool)
	for _, r := range g.characters {
		charSet[r] = true
	}

	for _, r := range g.rowChars {
		charSet[r] = true
	}

	for _, r := range g.colChars {
		charSet[r] = true
	}

	result := make([]rune, 0, len(charSet))
	for r := range charSet {
		result = append(result, r)
	}

	slices.Sort(result)

	return string(result)
}

// Bounds returns the screen bounds.
func (g *Grid) Bounds() image.Rectangle {
	return g.bounds
}

// Cells returns all cells with 3-char coordinates.
func (g *Grid) Cells() []*Cell {
	return g.cells
}

// Index returns the cell index map.
func (g *Grid) Index() map[string]*Cell {
	return g.index
}

// generateCellsWithRegions creates cells using spatial region logic.
// Each region (identified by first char) fills left-to-right, top-to-bottom.
// Handles variable label lengths (2, 3, or 4 chars) and distributes remainder pixels
// to ensure cells cover the entire screen bounds without gaps.
//
// New behavior: Regions are evenly distributed across the screen based on the number
// of available first characters (prefix characters). For example, with 6 prefix chars,
// the screen is divided into 6 equal regions arranged in an optimal grid layout.
//
// Parameters:
//   - prefixChars: characters used for region identification (first character of coordinate)
//   - prefixNumChars: number of prefix characters (typically 6)
//   - chars: full character set used for internal grid cells (second and third characters)
//   - rowChars, colChars: character sets for row/column labels
//   - numChars: total number of characters in the full character set
func generateCellsWithRegions(
	prefixChars string, prefixNumChars int,
	chars, rowChars, colChars []rune,
	numChars, gridCols, gridRows, labelLength int,
	bounds image.Rectangle,
	baseCellWidth, baseCellHeight, remainderWidth, remainderHeight int,
	logger *zap.Logger,
) []*Cell {
	logger.Debug("Generating cells with regions",
		zap.Int("num_chars", numChars),
		zap.Int("prefix_num_chars", prefixNumChars),
		zap.Int("grid_cols", gridCols),
		zap.Int("grid_rows", gridRows),
		zap.Int("label_length", labelLength))

	cells := make([]*Cell, gridCols*gridRows)
	cellIndex := 0

	// Calculate the optimal region layout based on prefixNumChars
	// This determines how many region rows and columns we need
	regionLayoutCols, regionLayoutRows := calculateRegionLayout(prefixNumChars, gridCols, gridRows, bounds)
	logger.Debug("Region layout calculated",
		zap.Int("region_layout_cols", regionLayoutCols),
		zap.Int("region_layout_rows", regionLayoutRows))

	// Calculate how many grid cells each region occupies
	// Using integer division - remainder will be distributed
	cellsPerRegionCol := gridCols / regionLayoutCols
	cellsPerRegionRow := gridRows / regionLayoutRows
	remainderRegionCols := gridCols % regionLayoutCols
	remainderRegionRows := gridRows % regionLayoutRows

	// Precompute x/y starts to avoid inner summation loops
	xStarts := make([]int, gridCols)
	yStarts := make([]int, gridRows)

	for colIndex := range xStarts {
		xStarts[colIndex] = bounds.Min.X + colIndex*baseCellWidth
		if colIndex < remainderWidth {
			xStarts[colIndex] += colIndex
		} else {
			xStarts[colIndex] += remainderWidth
		}
	}

	for rowIndex := range yStarts {
		yStarts[rowIndex] = bounds.Min.Y + rowIndex*baseCellHeight
		if rowIndex < remainderHeight {
			yStarts[rowIndex] += rowIndex
		} else {
			yStarts[rowIndex] += remainderHeight
		}
	}

	// Determine internal region dimensions based on label length
	var internalRegionCols, internalRegionRows int
	switch labelLength {
	case LabelLength2:
		internalRegionCols = len(colChars)
		internalRegionRows = 1
	case LabelLength3:
		internalRegionCols = len(colChars)
		internalRegionRows = len(rowChars)
	default: // 4 chars
		internalRegionCols = len(colChars)
		internalRegionRows = len(rowChars)
	}

	// Iterate through each region and fill it
	regionIndex := 0
	currentRegionRow := 0
	prefixRunes := []rune(prefixChars)

	for currentRegionRow < regionLayoutRows && regionIndex < prefixNumChars {
		currentRegionCol := 0

		for currentRegionCol < regionLayoutCols && regionIndex < prefixNumChars {
			// Determine region identifier character(s)
			var regionChar1 rune

			switch labelLength {
			case LabelLength2, LabelLength3:
				// For 2-char and 3-char labels: single character identifies the region
				regionChar1 = prefixRunes[regionIndex]
			default: // 4 chars
				// For 4-char labels with uniform region layout:
				// We have exactly prefixNumChars regions, so use single char for region identifier
				regionChar1 = prefixRunes[regionIndex]
			}

			// Calculate the grid cell range for this region
			regionStartCol := currentRegionCol*cellsPerRegionCol + gridMin(currentRegionCol, remainderRegionCols)
			regionStartRow := currentRegionRow*cellsPerRegionRow + gridMin(currentRegionRow, remainderRegionRows)

			regionEndCol := regionStartCol + cellsPerRegionCol
			if currentRegionCol < remainderRegionCols {
				regionEndCol++
			}

			regionEndRow := regionStartRow + cellsPerRegionRow
			if currentRegionRow < remainderRegionRows {
				regionEndRow++
			}

			// Ensure we don't exceed grid bounds
			regionEndCol = gridMin(regionEndCol, gridCols)
			regionEndRow = gridMin(regionEndRow, gridRows)

			// Calculate actual cells available in this region
			actualRegionCols := regionEndCol - regionStartCol
			actualRegionRows := regionEndRow - regionStartRow

			// Determine how many cells we can actually fill (limited by internal dimensions)
			fillCols := gridMin(internalRegionCols, actualRegionCols)
			fillRows := gridMin(internalRegionRows, actualRegionRows)

			// Fill cells in this region
			for rowIndex := range fillRows {
				for colIndex := range fillCols {
					globalCol := regionStartCol + colIndex
					globalRow := regionStartRow + rowIndex

					if globalCol >= gridCols || globalRow >= gridRows {
						continue
					}

					// Generate coordinate for this cell
					var coordinate string

					switch labelLength {
					case LabelLength2:
						var stringBuilder strings.Builder
						stringBuilder.Grow(StringBuilderGrow2)
						stringBuilder.WriteRune(regionChar1)
						stringBuilder.WriteRune(colChars[colIndex%len(colChars)])
						coordinate = stringBuilder.String()
					case LabelLength3:
						char2 := colChars[colIndex%len(colChars)]
						char3 := rowChars[rowIndex%len(rowChars)]

						var stringBuilder strings.Builder
						stringBuilder.Grow(StringBuilderGrow3)
						stringBuilder.WriteRune(regionChar1)
						stringBuilder.WriteRune(char2)
						stringBuilder.WriteRune(char3)
						coordinate = stringBuilder.String()
				default: // 4 chars
					// For 4-char labels with uniform layout: region + secondary + col + row
					// Use colIndex to determine secondary char to ensure uniqueness
					char2 := colChars[colIndex%len(colChars)]
					char3 := colChars[colIndex%len(colChars)]
					char4 := rowChars[rowIndex%len(rowChars)]

					var stringBuilder strings.Builder
					stringBuilder.Grow(StringBuilderGrow4)
					stringBuilder.WriteRune(regionChar1)
					stringBuilder.WriteRune(char2)
					stringBuilder.WriteRune(char3)
					stringBuilder.WriteRune(char4)
					coordinate = stringBuilder.String()
				}

					// Calculate cell dimensions
					cellWidth := baseCellWidth
					if globalCol < remainderWidth {
						cellWidth++
					}

					cellHeight := baseCellHeight
					if globalRow < remainderHeight {
						cellHeight++
					}

					xCoordinate := xStarts[globalCol]
					yCoordinate := yStarts[globalRow]

					cell := &Cell{
						coordinate: coordinate,
						bounds: image.Rect(
							xCoordinate, yCoordinate,
							xCoordinate+cellWidth, yCoordinate+cellHeight,
						),
						center: image.Point{
							X: xCoordinate + cellWidth/2,
							Y: yCoordinate + cellHeight/2,
						},
					}
					cells[cellIndex] = cell
					cellIndex++
				}
			}

			regionIndex++
			currentRegionCol++
		}
		currentRegionRow++
	}

	// Return only the filled portion of the slice
	return cells[:cellIndex]
}

// calculateRegionLayout determines the optimal grid layout for regions based on numChars.
// It finds the best (cols, rows) combination that minimizes aspect ratio deviation
// from a square and maximizes the number of regions that fit.
// For example, with 6 chars: prefers 3x2 or 2x3 over 6x1 or 1x6.
// The layout is adjusted based on screen aspect ratio:
//   - Landscape screens (width > height): prefers 2 rows × 3 cols layout
//   - Portrait screens (height > width): prefers 3 rows × 2 cols layout
func calculateRegionLayout(numChars, gridCols, gridRows int, bounds image.Rectangle) (int, int) {
	if numChars <= 0 {
		return 1, 1
	}

	// Determine if screen is portrait or landscape
	screenWidth := bounds.Dx()
	screenHeight := bounds.Dy()
	isPortrait := screenHeight > screenWidth

	// Find all divisors and valid layouts
	bestCols, bestRows := numChars, 1

	// Collect all valid layouts first
	type layout struct {
		cols, rows int
		score      float64
	}
	var validLayouts []layout

	// Try all possible layouts
	for cols := 1; cols <= numChars; cols++ {
		if numChars%cols == 0 {
			rows := numChars / cols

			// Check if this layout fits within our grid
			if cols > gridCols || rows > gridRows {
				continue
			}

			// Calculate aspect ratio score (prefer square-like regions)
			layoutAspect := float64(cols) / float64(rows)
			aspectDiff := math.Abs(layoutAspect - 1.0)

			// Also consider how well the regions fit the screen aspect ratio
			if gridCols > 0 && gridRows > 0 {
				screenAspect := float64(gridCols) / float64(gridRows)
				cellsPerRegionCol := float64(gridCols) / float64(cols)
				cellsPerRegionRow := float64(gridRows) / float64(rows)
				regionAspect := cellsPerRegionCol / cellsPerRegionRow
				fitScore := math.Abs(regionAspect - screenAspect)

				// Combined score: prefer square regions that fit screen well
				score := aspectDiff + fitScore*0.5

				validLayouts = append(validLayouts, layout{cols, rows, score})
			} else {
				validLayouts = append(validLayouts, layout{cols, rows, aspectDiff})
			}
		}
	}

	// Sort layouts by score
	slices.SortFunc(validLayouts, func(a, b layout) int {
		if a.score < b.score {
			return -1
		}
		if a.score > b.score {
			return 1
		}
		return 0
	})

	// Apply screen orientation preference
	// For portrait screens, prefer more rows; for landscape, prefer more cols
	for _, l := range validLayouts {
		if isPortrait && l.rows > l.cols {
			bestCols = l.cols
			bestRows = l.rows
			break
		}
		if !isPortrait && l.cols > l.rows {
			bestCols = l.cols
			bestRows = l.rows
			break
		}
	}

	// If no orientation-preferred layout found, use the best scored one
	if bestCols == numChars && bestRows == 1 && len(validLayouts) > 0 {
		bestCols = validLayouts[0].cols
		bestRows = validLayouts[0].rows
	}

	return bestCols, bestRows
}

// Candidate represents a valid grid configuration.
type Candidate struct {
	cols, rows   int
	cellW, cellH int
	score        float64
}

// calculateOptimalCellSizes determines optimal cell size constraints based on screen characteristics.
func calculateOptimalCellSizes(width, height int) (int, int) {
	screenArea := width * height
	screenAspect := float64(width) / float64(height)

	var minCellSize, maxCellSize int

	// Calculate optimal cell size ranges based on screen size and pixel density
	switch {
	case screenArea < SmallScreenArea:
		minCellSize = 30
		maxCellSize = 60
	case screenArea < MediumScreenArea:
		minCellSize = 30
		maxCellSize = 80
	case screenArea < LargeScreenArea:
		minCellSize = 40
		maxCellSize = 100
	default:
		minCellSize = 50
		maxCellSize = 120
	}

	// Adjust cell size constraints for extreme aspect ratios
	if screenAspect > ExtremeAspectRatioHigh || screenAspect < ExtremeAspectRatioLow {
		maxCellSize = int(float64(maxCellSize) * AspectRatioAdjustment)
	}

	return minCellSize, maxCellSize
}

// calculateLabelLength determines the optimal label length based on total cells and available characters.
func calculateLabelLength(totalCells, numChars, numRowChars, numColChars int) int {
	// If custom row/col labels are provided (numRowChars/numColChars != numChars), use more labels
	if numRowChars != numChars || numColChars != numChars {
		max2Char := numChars * numColChars

		max3Char := numChars * numColChars * numRowChars
		switch {
		case totalCells <= max2Char:
			return LabelLength2
		case totalCells <= max3Char:
			return LabelLength3
		default:
			return LabelLength4
		}
	}
	// Default logic when using characters for everything
	switch {
	case totalCells <= numChars*numChars:
		return LabelLength2
	case totalCells <= numChars*numChars*numChars:
		return LabelLength3
	default:
		return LabelLength4
	}
}

// selectBestCandidate picks the candidate with the best (lowest) score.
func selectBestCandidate(
	candidates []Candidate,
	width, height, minCellSize, maxCellSize int,
) (int, int) {
	var gridCols, gridRows int

	if len(candidates) > 0 {
		best := candidates[0]
		for _, cand := range candidates[1:] {
			if cand.score < best.score {
				best = cand
			}
		}

		gridCols = best.cols
		gridRows = best.rows
	} else {
		// Fallback: if no valid candidates, use simple best-fit approach
		findBestFit := func(dimension, minSize, maxSize int) int {
			count := gridMax(dimension/minSize, 1)
			for dimension/count > maxSize {
				count++
			}

			return count
		}
		gridCols = findBestFit(width, minCellSize, maxCellSize)
		gridRows = findBestFit(height, minCellSize, maxCellSize)
	}

	return gridCols, gridRows
}

// findValidGridConfigurations searches through all valid grid configurations.
// Evaluates combinations of columns and rows within cell size constraints,
// calculating aspect ratio scores to find grids that produce square-like cells.
// Returns candidates sorted by score (lower is better).
func findValidGridConfigurations(width, height, minCellSize, maxCellSize int) []Candidate {
	var (
		candidates []Candidate
		mutex      sync.Mutex
	)

	// Calculate search ranges
	minCols := max(width/maxCellSize, 1)
	maxCols := max(width/minCellSize, 1)

	minRows := max(height/maxCellSize, 1)
	maxRows := max(height/minCellSize, 1)

	// Use WaitGroup for parallel computation
	var waitGroup sync.WaitGroup

	// Search through all valid grid configurations within constraints
	for colIndex := maxCols; colIndex >= minCols && colIndex >= 1; colIndex-- {
		waitGroup.Add(1)

		go func(col int) {
			defer waitGroup.Done()

			var localCandidates []Candidate

			cellWidth := width / col
			if cellWidth < minCellSize || cellWidth > maxCellSize {
				return
			}

			for rowIndex := maxRows; rowIndex >= minRows && rowIndex >= 1; rowIndex-- {
				cellHeight := height / rowIndex
				if cellHeight < minCellSize || cellHeight > maxCellSize {
					continue
				}

				// Calculate how square the cells are (aspect ratio deviation from 1.0)
				cellAspect := float64(cellWidth) / float64(cellHeight)

				aspectDiff := cellAspect - 1.0
				if aspectDiff < 0 {
					aspectDiff = -aspectDiff
				}

				// Prefer configurations with more cells for finer precision
				totalCells := float64(col * rowIndex)
				maxCells := float64(maxCols * maxRows)
				cellScore := (maxCells - totalCells) / maxCells * ScoreWeight

				aspectScore := aspectDiff + cellScore

				cand := Candidate{
					cols:  col,
					rows:  rowIndex,
					cellW: cellWidth,
					cellH: cellHeight,
					score: aspectScore,
				}

				localCandidates = append(localCandidates, cand)
			}

			mutex.Lock()

			candidates = append(candidates, localCandidates...)

			mutex.Unlock()
		}(colIndex)
	}

	waitGroup.Wait()

	return candidates
}

// AllCells returns all grid cells.
func (g *Grid) AllCells() []*Cell {
	return g.cells
}

// CellByCoordinate returns the cell for a given coordinate. (2, 3, or 4 characters).
func (g *Grid) CellByCoordinate(coordinate string) *Cell {
	coordinate = strings.ToUpper(coordinate)

	if g.index != nil {
		if cell, ok := g.index[coordinate]; ok {
			return cell
		}
	}

	for _, cell := range g.cells {
		if cell.Coordinate() == coordinate {
			return cell
		}
	}

	return nil
}

// HasCoordinatePrefix returns true if any coordinate starts with the given prefix.
func (g *Grid) HasCoordinatePrefix(prefix string) bool {
	prefix = strings.ToUpper(prefix)

	return g.prefixes[prefix]
}

// buildPrefixIndex creates a map of all coordinate prefixes for fast lookup.
func buildPrefixIndex(cells []*Cell) map[string]bool {
	prefixes := make(map[string]bool)
	for _, cell := range cells {
		coord := cell.Coordinate()
		for i := 1; i <= len(coord); i++ {
			prefixes[coord[:i]] = true
		}
	}

	return prefixes
}

// CalculateOptimalGrid calculates optimal character count for coverage.
func CalculateOptimalGrid(characters string) (int, int) {
	// For flat 3-char grid, we don't use rows/cols
	// Just return sensible defaults (will be ignored)
	numChars := len(characters)
	if numChars < MinCharactersLength {
		numChars = 9
	}

	return numChars, numChars
}

func gridMax(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func gridMin(a, b int) int {
	if a < b {
		return a
	}

	return b
}

package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	chromath "github.com/jkl1337/go-chromath"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// BeadConfig configures a bead color
type BeadConfig struct {
	R, G, B     uint8
	GreyShade   bool
	Translucent bool
	Flourescent bool
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "beadmachine file.jpg",
		Short: "Bead pattern creator",
		Run:   startBeadMachine,
	}

	rootCmd.Flags().BoolP("verbose", "v", false, "verbose output")

	// files
	rootCmd.Flags().StringP("input", "i", "", "image to process")
	rootCmd.Flags().StringP("output", "o", "", "output filename for the converted PNG image")
	rootCmd.Flags().StringP("html", "l", "", "output filename for a HTML based bead pattern file")
	rootCmd.Flags().StringP("palette", "p", "colors_hama.json", "filename of the bead palette")

	// dimensions
	rootCmd.Flags().IntP("width", "w", 0, "resize image to width in pixel")
	rootCmd.Flags().IntP("height", "e", 0, "resize image to height in pixel")
	rootCmd.Flags().IntP("boardswidth", "x", 0, "resize image to width in amount of boards")
	rootCmd.Flags().IntP("boardsheight", "y", 0, "resize image to height in amount of boards")
	rootCmd.Flags().IntP("boarddimension", "d", 20, "dimension of a board")

	// bead types
	rootCmd.Flags().BoolP("beadstyle", "b", false, "make output file look like a beads board")
	rootCmd.Flags().BoolP("translucent", "t", false, "include translucent colors for the conversion")
	rootCmd.Flags().BoolP("flourescent", "f", false, "include flourescent colors for the conversion")

	// filters
	rootCmd.Flags().BoolP("nocolormatching", "n", false, "skip the bead color matching")
	rootCmd.Flags().BoolP("grey", "g", false, "convert the image to greyscale")
	rootCmd.Flags().Float64P("blur", "", 0.0, "apply blur filter (0.0 - 10.0)")
	rootCmd.Flags().Float64P("sharpen", "", 0.0, "apply sharpen filter (0.0 - 10.0)")
	rootCmd.Flags().Float64P("gamma", "", 0.0, "apply gamma correction (0.0 - 10.0)")
	rootCmd.Flags().Float64P("contrast", "", 0.0, "apply contrast adjustment (-100 - 100)")
	rootCmd.Flags().Float64P("brightness", "", 0.0, "apply brightness adjustment (-100 - 100)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("ERROR: %v\n", err)
	}
}

func startBeadMachine(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		_ = cmd.Help()
		return
	}

	logger := logger(cmd)

	inputFileName, _ := cmd.Flags().GetString("config")
	inputImage, err := readImageFile(inputFileName)
	if err != nil {
		logger.Error("Reading image file failed", zap.Error(err))
		return
	}

	imageBounds := inputImage.Bounds()
	logger.Info("Image pixels",
		zap.Int("width", imageBounds.Dx()),
		zap.Int("height", imageBounds.Dy()))

	inputImage = applyFilters(cmd, inputImage) // apply filters before resizing for better results

	// resize the image if needed
	boardDimension, _ := cmd.Flags().GetInt("boarddimension")

	newWidth, _ := cmd.Flags().GetInt("width")
	newWidthBoards, _ := cmd.Flags().GetInt("boardswidth")
	if newWidthBoards > 0 { // a given boards number overrides a possible given pixel number
		newWidth = newWidthBoards * boardDimension
	}

	newHeight, _ := cmd.Flags().GetInt("height")
	newHeightBoards, _ := cmd.Flags().GetInt("boardsheight")
	if newHeightBoards > 0 {
		newHeight = newHeightBoards * boardDimension
	}
	resized := false
	if newWidth > 0 || newHeight > 0 {
		inputImage = imaging.Resize(inputImage, newWidth, newHeight, imaging.Lanczos)
		imageBounds = inputImage.Bounds()
		resized = true
	}

	logger.Info("Bead board used",
		zap.Int("width", calculateBeadBoardsNeeded(imageBounds.Dx())),
		zap.Int("height", calculateBeadBoardsNeeded(imageBounds.Dy())))
	logger.Info("Bead board measurement in cm",
		zap.Float64("width", float64(imageBounds.Dx())*0.5),
		zap.Float64("height", float64(imageBounds.Dy())*0.5))

	beadModeImageBounds := imageBounds
	beadStyle, _ := cmd.Flags().GetBool("beadstyle")
	if beadStyle { // each pixel will be a bead of 8x8 pixel
		beadModeImageBounds.Max.X *= 8
		beadModeImageBounds.Max.Y *= 8
	}
	outputImage := image.NewRGBA(beadModeImageBounds)

	if resized || beadStyle {
		logger.Info("Output image pixels",
			zap.Int("width", imageBounds.Dx()),
			zap.Int("height", imageBounds.Dy()))
	}

	noColorMatching, _ := cmd.Flags().GetBool("nocolormatching")
	if noColorMatching {
		for y := imageBounds.Min.Y; y < imageBounds.Max.Y; y++ {
			for x := imageBounds.Min.X; x < imageBounds.Max.X; x++ {
				pixelColor := inputImage.At(x, y)
				r, g, b, _ := pixelColor.RGBA()
				pixelRGBA := color.RGBA{uint8(r), uint8(g), uint8(b), 255} // A 255 = no transparency
				outputImage.SetRGBA(x, y, pixelRGBA)
			}
		}
	} else {
		paletteFileName, _ := cmd.Flags().GetString("palette")
		startTime := time.Now()
		if err := processImage(cmd, logger, imageBounds, inputImage, outputImage, paletteFileName); err != nil {
			logger.Error("Processing image failed", zap.Error(err))
			return
		}
		elapsedTime := time.Since(startTime)
		logger.Info("Image processed", zap.Duration("duration", elapsedTime))
	}

	outputFileName, _ := cmd.Flags().GetString("output")
	imageWriter, err := os.Create(outputFileName)
	if err != nil {
		logger.Error("Opening output image file failed", zap.Error(err))
		return
	}
	defer imageWriter.Close()

	png.Encode(imageWriter, outputImage)
}

func logger(cmd *cobra.Command) *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.Development = false
	config.DisableCaller = true
	config.DisableStacktrace = true

	level := config.Level
	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		level.SetLevel(zap.DebugLevel)
	} else {
		level.SetLevel(zap.InfoLevel)
	}

	log, _ := config.Build()
	return log
}

var (
	// color conversion variables
	targetIlluminant = &chromath.IlluminantRefD50
	labTransformer   = chromath.NewLabTransformer(targetIlluminant)
	rgbTransformer   = chromath.NewRGBTransformer(&chromath.SpaceSRGB, &chromath.AdaptationBradford, targetIlluminant, &chromath.Scaler8bClamping, 1.0, nil)
	beadFillPixel    = color.RGBA{225, 225, 225, 255} // light grey

	// conversion synchronisation variables
	colorMatchCache     = make(map[color.Color]string)
	colorMatchCacheLock sync.RWMutex
	rgbLabCache         = make(map[color.Color]chromath.Lab)
	rgbLabCacheLock     sync.RWMutex
	beadStatsDone       = make(chan struct{})
)

// calculateBeadUsage calculates the bead usage
func calculateBeadUsage(logger *zap.Logger, beadUsageChan <-chan string) {
	colorUsageCounts := make(map[string]int)

	for beadName := range beadUsageChan {
		colorUsageCounts[beadName]++
	}

	logger.Info("Bead colors", zap.Int("count", len(colorUsageCounts)))
	for usedColor, count := range colorUsageCounts {
		logger.Info("Beads used", zap.String("color", usedColor), zap.Int("count", count))
	}
	beadStatsDone <- struct{}{}
}

// calculateBeadBoardsNeeded calculates the needed bead boards based on the standard size of 29 beads for a dimension
func calculateBeadBoardsNeeded(dimension int) int {
	neededFloat := float64(dimension) / 29
	neededFloat = math.Floor(neededFloat + .5)
	return int(neededFloat) // round up
}

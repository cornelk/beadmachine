package main

import (
	"fmt"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"

	chromath "github.com/jkl1337/go-chromath"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

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

	inputFileName, _ := cmd.Flags().GetString("input")
	outputFileName, _ := cmd.Flags().GetString("output")
	htmlFileName, _ := cmd.Flags().GetString("html")
	paletteFileName, _ := cmd.Flags().GetString("palette")

	width, _ := cmd.Flags().GetInt("width")
	height, _ := cmd.Flags().GetInt("height")
	newWidthBoards, _ := cmd.Flags().GetInt("boardswidth")
	newHeightBoards, _ := cmd.Flags().GetInt("boardsheight")
	boardDimension, _ := cmd.Flags().GetInt("boarddimension")

	beadStyle, _ := cmd.Flags().GetBool("beadstyle")
	useTranslucent, _ := cmd.Flags().GetBool("translucent")
	useFlourescent, _ := cmd.Flags().GetBool("flourescent")

	noColorMatching, _ := cmd.Flags().GetBool("nocolormatching")
	greyScale, _ := cmd.Flags().GetBool("grey")
	filterBlur, _ := cmd.Flags().GetFloat64("blur")
	filterSharpen, _ := cmd.Flags().GetFloat64("sharpen")
	filterGamma, _ := cmd.Flags().GetFloat64("gamma")
	filterContrast, _ := cmd.Flags().GetFloat64("contrast")
	filterBrightness, _ := cmd.Flags().GetFloat64("brightness")

	m := &beadMachine{
		logger: logger,

		colorMatchCache: make(map[color.Color]string),
		rgbLabCache:     make(map[color.Color]chromath.Lab),
		beadStatsDone:   make(chan struct{}),

		labTransformer: chromath.NewLabTransformer(&chromath.IlluminantRefD50),
		rgbTransformer: chromath.NewRGBTransformer(&chromath.SpaceSRGB, &chromath.AdaptationBradford, &chromath.IlluminantRefD50, &chromath.Scaler8bClamping, 1.0, nil),
		beadFillPixel:  color.RGBA{225, 225, 225, 255}, // light grey

		inputFileName:   inputFileName,
		outputFileName:  outputFileName,
		paletteFileName: paletteFileName,
		htmlFileName:    htmlFileName,

		boardDimension: boardDimension,
		width:          width,
		boardsWidth:    newWidthBoards,
		height:         height,
		boardsHeight:   newHeightBoards,

		beadStyle:       beadStyle,
		noColorMatching: noColorMatching,
		greyScale:       greyScale,
		translucent:     useTranslucent,
		flourescent:     useFlourescent,

		blur:       filterBlur,
		sharpen:    filterSharpen,
		gamma:      filterGamma,
		contrast:   filterContrast,
		brightness: filterBrightness,
	}
	m.process()
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

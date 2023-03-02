// Package main implements a Bead pattern creator.
package main

import (
	"fmt"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"os"

	"github.com/alexflint/go-arg"
	"github.com/cornelk/gotokit/env"
	"github.com/cornelk/gotokit/log"
	chromath "github.com/jkl1337/go-chromath"
)

const toolName = "beadmachine"

type arguments struct {
	Verbose bool `arg:"-v,--verbose" help:"verbose output"`

	// files
	InputFileName   string `arg:"-i,--input" help:"image to process"`
	OutputFileName  string `arg:"-o,--output" help:"output filename for the converted PNG image"`
	HTMLFileName    string `arg:"-l,--html" help:"output filename for a HTML based bead pattern file"`
	PaletteFileName string `arg:"-p,--palette" help:"filename of the bead palette" default:"colors_hama.json"`

	// dimensions
	Width          int `arg:"-w,--width" help:"resize image to width in pixel"`
	Height         int `arg:"-e,--height" help:"resize image to height in pixel"`
	BoardWidth     int `arg:"-x,--boardwidth" help:"resize image to width in amount of boards"`
	BoardHeight    int `arg:"-y,--boardheight" help:"resize image to height in amount of boards"`
	BoardDimension int `arg:"-y,--boarddimension" help:"dimension of a board" default:"20"`

	// bead types
	BeadStyle   bool `arg:"-b,--beadstyle" help:"make output file look like a beads board"`
	Translucent bool `arg:"-t,--translucent" help:"include translucent colors for the conversion"`
	fluorescent bool `arg:"-f,--fluorescent" help:"include fluorescent colors for the conversion"`

	// filters
	NoColorMatching bool    `arg:"-n,--nocolormatching" help:"skip the bead color matching"`
	GreyScale       bool    `arg:"-g,--grey" help:"convert the image to greyscale"`
	Blur            float64 `arg:"--blur" help:"apply blur filter (0.0 - 10.0)"`
	Sharpen         float64 `arg:"--sharpen" help:"apply sharpen filter (0.0 - 10.0)"`
	Gamma           float64 `arg:"--gamma" help:"apply gamma filter (0.0 - 10.0)"`
	Contrast        float64 `arg:"--contrast" help:"apply contrast adjustment (-100 - 100)"`
	Brightness      float64 `arg:"--brightness" help:"apply brightness adjustment (-100 - 100)"`
}

func (arguments) Description() string {
	return "Bead pattern creator.\n"
}

func main() {
	var args arguments
	arg.MustParse(&args)

	if err := run(args); err != nil {
		fmt.Printf("Execution error: %s\n", err)
		os.Exit(1)
	}
}

func run(args arguments) error {
	if args.Verbose {
		log.SetDefaultLevel(log.DebugLevel)
	}
	logger, err := createLogger()
	if err != nil {
		return fmt.Errorf("creating logger: %w", err)
	}

	m := &beadMachine{
		logger: logger,

		colorMatchCache: make(map[color.Color]string),
		rgbLabCache:     make(map[color.Color]chromath.Lab),
		beadStatsDone:   make(chan struct{}),

		labTransformer: chromath.NewLabTransformer(&chromath.IlluminantRefD50),
		rgbTransformer: chromath.NewRGBTransformer(&chromath.SpaceSRGB, &chromath.AdaptationBradford, &chromath.IlluminantRefD50, &chromath.Scaler8bClamping, 1.0, nil),
		beadFillPixel: color.RGBA{
			R: 225,
			G: 225,
			B: 225,
			A: 255}, // light grey

		inputFileName:   args.InputFileName,
		outputFileName:  args.OutputFileName,
		paletteFileName: args.PaletteFileName,
		htmlFileName:    args.HTMLFileName,

		boardDimension: args.BoardDimension,
		width:          args.Width,
		boardsWidth:    args.BoardWidth,
		height:         args.Height,
		boardsHeight:   args.BoardHeight,

		beadStyle:       args.BeadStyle,
		noColorMatching: args.NoColorMatching,
		greyScale:       args.GreyScale,
		translucent:     args.Translucent,
		fluorescent:     args.fluorescent,

		blur:       args.Blur,
		sharpen:    args.Sharpen,
		gamma:      args.Gamma,
		contrast:   args.Contrast,
		brightness: args.Brightness,
	}
	return m.process()
}

func createLogger() (*log.Logger, error) {
	logCfg, err := log.ConfigForEnv(env.Development)
	if err != nil {
		return nil, fmt.Errorf("initializing log config: %w", err)
	}
	logCfg.JSONOutput = false
	logCfg.CallerInfo = false

	logger, err := log.NewWithConfig(logCfg)
	if err != nil {
		return nil, fmt.Errorf("initializing logger: %w", err)
	}
	logger = logger.Named(toolName)
	return logger, nil
}

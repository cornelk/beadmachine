package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"runtime"
	"sync"
	"time"

	_ "image/gif"
	_ "image/jpeg"

	"github.com/disintegration/imaging"
	chromath "github.com/jkl1337/go-chromath"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

// RGB is a simple RGB struct to define a color
type RGB struct {
	R, G, B uint8
}

// BeadConfig configures a bead color
type BeadConfig struct {
	R, G, B     uint8
	GreyShade   bool
	Translucent bool
	Flourescent bool
}

var (
	// general
	verbose = kingpin.Flag("verbose", "Enable verbose output.").Short('v').Bool()

	// files
	inputFileName   = kingpin.Flag("input", "Filename of image to process.").Short('i').Required().String()
	outputFileName  = kingpin.Flag("output", "Output filename for the converted PNG image.").Short('o').PlaceHolder("OUTPUT.png").Required().String()
	htmlFileName    = kingpin.Flag("html", "Output filename for a HTML based bead pattern file.").Short('l').String()
	paletteFileName = kingpin.Flag("palette", "Filename of the bead palette.").Short('p').Default("colors_hama.json").String()

	// dimensions
	newWidth        = kingpin.Flag("width", "Resize image to width in pixel.").Short('w').Default("0").Int()
	newHeight       = kingpin.Flag("height", "Resize image to height in pixel.").Short('h').Default("0").Int()
	newWidthBoards  = kingpin.Flag("boardswidth", "Resize image to width in amount of boards.").Short('x').Default("0").Int()
	newHeightBoards = kingpin.Flag("boardsheight", "Resize image to height in amount of boards.").Short('y').Default("0").Int()
	boardDimension  = kingpin.Flag("boarddimension", "Dimension of a board.").Short('d').Default("29").Int()

	// bead types
	beadStyle      = kingpin.Flag("bead", "Make output file look like a beads board.").Short('b').Bool()
	useTranslucent = kingpin.Flag("translucent", "Include translucent colors for the conversion.").Short('t').Bool()
	useFlourescent = kingpin.Flag("flourescent", "Include flourescent colors for the conversion.").Short('f').Bool()

	// filters
	noColorMatching  = kingpin.Flag("nocolormatching", "Skip the bead color matching.").Short('n').Bool()
	greyScale        = kingpin.Flag("grey", "Convert the image to greyscale.").Bool()
	filterBlur       = kingpin.Flag("blur", "Apply blur filter (0.0 - 10.0).").Float()
	filterSharpen    = kingpin.Flag("sharpen", "Apply sharpen filter (0.0 - 10.0).").Float()
	filterGamma      = kingpin.Flag("gamma", "Apply gamma correction (0.0 - 10.0).").Float()
	filterContrast   = kingpin.Flag("contrast", "Apply contrast adjustment (-100 - 100).").Float()
	filterBrightness = kingpin.Flag("brightness", "Apply brightness adjustment (-100 - 100).").Float()

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
func calculateBeadUsage(beadUsageChan <-chan string) {
	colorUsageCounts := make(map[string]int)

	for beadName := range beadUsageChan {
		colorUsageCounts[beadName]++
	}

	fmt.Printf("Bead colors used: %v\n", len(colorUsageCounts))
	for usedColor, count := range colorUsageCounts {
		fmt.Printf("Beads used '%s': %v\n", usedColor, count)
	}
	beadStatsDone <- struct{}{}
}

// calculateBeadBoardsNeeded calculates the needed bead boards based on the standard size of 29 beads for a dimension
func calculateBeadBoardsNeeded(dimension int) int {
	neededFloat := float64(dimension) / 29
	neededFloat = math.Floor(neededFloat + .5)
	return int(neededFloat) // round up
}

func main() {
	kingpin.CommandLine.Help = "Bead pattern creator."
	kingpin.Parse()

	cpuCount := runtime.NumCPU()
	runtime.GOMAXPROCS(cpuCount) // use all cores for parallelism

	inputImage := readImageFile(*inputFileName)
	imageBounds := inputImage.Bounds()
	fmt.Printf("Input image width: %v, height: %v\n", imageBounds.Dx(), imageBounds.Dy())

	inputImage = applyFilters(inputImage) // apply filters before resizing for better results

	// resize the image if needed
	if *newWidthBoards > 0 { // a given boards number overrides a possible given pixel number
		*newWidth = *newWidthBoards * *boardDimension
	}
	if *newHeightBoards > 0 {
		*newHeight = *newHeightBoards * *boardDimension
	}
	resized := false
	if *newWidth > 0 || *newHeight > 0 {
		inputImage = imaging.Resize(inputImage, *newWidth, *newHeight, imaging.Lanczos)
		imageBounds = inputImage.Bounds()
		resized = true
	}

	fmt.Printf("Bead boards width: %v, height: %v\n", calculateBeadBoardsNeeded(imageBounds.Dx()), calculateBeadBoardsNeeded(imageBounds.Dy()))
	fmt.Printf("Beads width: %v cm, height: %v cm\n", float64(imageBounds.Dx())*0.5, float64(imageBounds.Dy())*0.5)

	beadModeImageBounds := imageBounds
	if *beadStyle { // each pixel will be a bead of 8x8 pixel
		beadModeImageBounds.Max.X *= 8
		beadModeImageBounds.Max.Y *= 8
	}
	outputImage := image.NewRGBA(beadModeImageBounds)

	if resized || *beadStyle {
		fmt.Printf("Output image pixel width: %v, height: %v\n", imageBounds.Dx(), imageBounds.Dy())
	}

	if *noColorMatching {
		for y := imageBounds.Min.Y; y < imageBounds.Max.Y; y++ {
			for x := imageBounds.Min.X; x < imageBounds.Max.X; x++ {
				pixelColor := inputImage.At(x, y)
				r, g, b, _ := pixelColor.RGBA()
				pixelRGBA := color.RGBA{uint8(r), uint8(g), uint8(b), 255} // A 255 = no transparency
				outputImage.SetRGBA(x, y, pixelRGBA)
			}
		}
	} else {
		startTime := time.Now()
		processImage(imageBounds, inputImage, outputImage, *paletteFileName)
		elapsedTime := time.Since(startTime)
		fmt.Printf("Image processed in %s\n", elapsedTime)
	}

	imageWriter, err := os.Create(*outputFileName)
	if err != nil {
		fmt.Printf("Opening image output file failed: %v\n", err)
		os.Exit(1)
	}
	defer imageWriter.Close()

	png.Encode(imageWriter, outputImage)
}

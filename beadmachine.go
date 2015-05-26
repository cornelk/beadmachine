package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "image/gif"
	_ "image/jpeg"

	"github.com/disintegration/imaging"
	chromath "github.com/jkl1337/go-chromath"
	"github.com/jkl1337/go-chromath/deltae"
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
	beadStatsDone       = make(chan struct{})
)

// LoadPalette loads a palette from a json file and returns a LAB color palette
func LoadPalette(fileName string) (map[string]BeadConfig, map[chromath.Lab]string) {
	cfgData, err := ioutil.ReadFile(fileName)
	if err != nil {
		fmt.Printf("File error: %v\n", err)
		os.Exit(1)
	}

	cfg := make(map[string]BeadConfig)
	err = json.Unmarshal(cfgData, &cfg)
	if err != nil {
		fmt.Printf("Config json error: %v\n", err)
		os.Exit(1)
	}

	cfgLab := make(map[chromath.Lab]string)
	for beadName, rgbOriginal := range cfg {
		if *greyScale == true && rgbOriginal.GreyShade == false { // only process grey shades in greyscale mode
			continue
		}
		if *useTranslucent == false && rgbOriginal.Translucent == true { // only process translucent in translucent mode
			continue
		}
		if *useFlourescent == false && rgbOriginal.Flourescent == true { // only process flourescent in flourescent mode
			continue
		}

		rgb := chromath.RGB{float64(rgbOriginal.R), float64(rgbOriginal.G), float64(rgbOriginal.B)}
		xyz := rgbTransformer.Convert(rgb)
		lab := labTransformer.Invert(xyz)
		cfgLab[lab] = beadName
		//fmt.Printf("Loaded bead: '%v', RGB: %v, Lab: %v\n", beadName, rgb, lab)
	}

	return cfg, cfgLab
}

// FindSimilarColor finds the most similar color from bead palette to the given pixel
func FindSimilarColor(cfgLab map[chromath.Lab]string, pixel color.Color) (string, bool) {
	colorMatchCacheLock.RLock()
	match, found := colorMatchCache[pixel]
	colorMatchCacheLock.RUnlock()
	if found {
		return match, true
	}

	r, g, b, _ := pixel.RGBA()
	rgb := chromath.RGB{float64(uint8(r)), float64(uint8(g)), float64(uint8(b))}
	xyz := rgbTransformer.Convert(rgb)
	labPixel := labTransformer.Invert(xyz)

	var minDistance float64
	var bestBeadMatch string

	for lab, beadName := range cfgLab {
		distance := deltae.CIE2000(lab, labPixel, &deltae.KLChDefault)
		if len(bestBeadMatch) == 0 || distance < minDistance {
			minDistance = distance
			bestBeadMatch = beadName
		}
		//fmt.Printf("Match: %v with distance: %v\n", beadName, distance)
	}

	//fmt.Printf("Best match: %v with distance: %v\n", bestBeadMatch, minDistance)
	return bestBeadMatch, false
}

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

// setOutputImagePixel sets a pixel in the output image or draws a bead in beadStyle mode
func setOutputImagePixel(outputImage *image.RGBA, coordinates image.Point, bead BeadConfig) {
	rgbaMatch := color.RGBA{bead.R, bead.G, bead.B, 255} // A 255 = no transparency
	if *beadStyle {
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				if (x%7 == 0 && y%7 == 0) || (x > 2 && x < 5 && y > 2 && y < 5) { // all corner pixel + 2x2 in center
					outputImage.SetRGBA((coordinates.X*8)+x, (coordinates.Y*8)+y, beadFillPixel)
				} else {
					outputImage.SetRGBA((coordinates.X*8)+x, (coordinates.Y*8)+y, rgbaMatch)
				}
			}
		}
	} else {
		outputImage.SetRGBA(coordinates.X, coordinates.Y, rgbaMatch)
	}
}

// calculateBeadBoardsNeeded calculates the needed bead boards based on the standard size of 29 beads for a dimension
func calculateBeadBoardsNeeded(dimension int) int {
	neededFloat := float64(dimension) / 29
	neededFloat = math.Floor(neededFloat + .5)
	return int(neededFloat) // round up
}

// writeHTMLBeadInstructionFile writes a HTML file with instructions on how to make the bead based image
func writeHTMLBeadInstructionFile(fileName string, outputImageBounds image.Rectangle, outputImage *image.RGBA, outputImageBeadNames []string) {
	htmlFile, err := os.Create(fileName)
	if err != nil {
		fmt.Printf("Opening HTML output file failed: %v\n", err)
		os.Exit(1)
	}

	w := bufio.NewWriter(htmlFile)
	w.WriteString("<html>\n<head>\n")
	w.WriteString("<style type=\"text/css\">\n")
	w.WriteString(".lb { border-left: 2px solid black !important; }\n")
	w.WriteString(".rb { border-right: 2px solid black !important; }\n")
	w.WriteString(".tb td { border-top: 2px solid black !important; }\n")
	w.WriteString(".bb td { border-bottom: 2px solid black !important; }\n")
	w.WriteString("</style>\n</head>\n<body>\n")
	w.WriteString("<table style=\"border-spacing: 0px;\">\n")

	for y := outputImageBounds.Min.Y; y < outputImageBounds.Max.Y; y++ {
		w.WriteString("<tr")
		if y == 0 { // // draw top bead board horizontal border
			w.WriteString(" class=\"tb\"")
		}
		w.WriteString(">")

		// write a line with colored cells
		for x := outputImageBounds.Min.X; x < outputImageBounds.Max.X; x++ {
			pixel := outputImage.RGBAAt(x, y)
			colorstring := fmt.Sprintf("#%02X%02X%02X", pixel.R, pixel.G, pixel.B)

			w.WriteString("<td bgcolor=\"" + colorstring + "\"")
			if x == 0 {
				w.WriteString(" class=\"lb\"") // draw left bead board vertical border
			} else {
				if (x+1)%*boardDimension == 0 { // draw bead board vertical border
					w.WriteString(" class=\"rb\"")
				}
			}
			w.WriteString(">&nbsp;</td>")
		}
		w.WriteString("</tr>\n")

		w.WriteString("<tr")
		if y > 0 && (y+1)%*boardDimension == 0 { // draw bead board horizontal border
			w.WriteString(" class=\"bb\"")
		}
		w.WriteString(">")

		// write a line with bead names
		for x := outputImageBounds.Min.X; x < outputImageBounds.Max.X; x++ {
			beadName := outputImageBeadNames[x+y*outputImageBounds.Max.X]
			shortName := strings.Split(beadName, " ")

			w.WriteString("<td")
			if x == 0 {
				w.WriteString(" class=\"lb\"") // draw left bead board vertical border
			} else {
				if (x+1)%*boardDimension == 0 { // draw bead board vertical border
					w.WriteString(" class=\"rb\"")
				}
			}
			w.WriteString(">" + shortName[0] + "</td>") // only print first part of name
		}
		w.WriteString("</tr>\n")
	}

	w.WriteString("</table>\n</body>\n</html>\n")
	w.Flush()
	htmlFile.Close()
}

// applyfilters will apply all filters that were enabled to the input image
func applyFilters(inputImage image.Image) image.Image {
	filteredImage := inputImage

	if *greyScale {
		filteredImage = imaging.Grayscale(filteredImage)
	}
	if *filterBlur != 0.0 {
		filteredImage = imaging.Blur(filteredImage, *filterBlur)
	}
	if *filterSharpen != 0.0 {
		filteredImage = imaging.Sharpen(filteredImage, *filterSharpen)
	}
	if *filterGamma != 0.0 {
		filteredImage = imaging.AdjustGamma(filteredImage, *filterGamma)
	}
	if *filterContrast != 0.0 {
		filteredImage = imaging.AdjustContrast(filteredImage, *filterContrast)
	}
	if *filterBrightness != 0.0 {
		filteredImage = imaging.AdjustBrightness(filteredImage, *filterBrightness)
	}

	return filteredImage
}

func main() {
	kingpin.CommandLine.Help = "Bead pattern creator."
	kingpin.Parse()

	cpuCount := runtime.NumCPU()
	runtime.GOMAXPROCS(cpuCount) // use all cores for parallelism

	beadConfig, beadLab := LoadPalette(*paletteFileName)

	imageReader, err := os.Open(*inputFileName)
	if err != nil {
		fmt.Printf("Opening input image file failed: %v\n", err)
		os.Exit(1)
	}
	defer imageReader.Close()

	inputImage, _, err := image.Decode(imageReader)
	imageBounds := inputImage.Bounds()
	fmt.Printf("Input image width: %v, height: %v\n", imageBounds.Dx(), imageBounds.Dy())

	inputImage = applyFilters(inputImage) // apply filters before resizing for better results

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
	pixelCount := imageBounds.Dx() * imageBounds.Dy()

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

	beadUsageChan := make(chan string, pixelCount)
	workQueueChan := make(chan image.Point, cpuCount*2)
	workDone := make(chan struct{})

	var outputImageBeadNames []string
	if len(*htmlFileName) > 0 {
		outputImageBeadNames = make([]string, pixelCount)
	}

	var pixelWaitGroup sync.WaitGroup
	pixelWaitGroup.Add(pixelCount)

	go func() { // pixel channel worker goroutine
		for {
			select {
			case pixel := <-workQueueChan:
				go func(pixel image.Point) { // pixel processing goroutine
					defer pixelWaitGroup.Done()
					oldPixel := inputImage.At(pixel.X, pixel.Y)
					beadName, cached := FindSimilarColor(beadLab, oldPixel)
					beadUsageChan <- beadName
					if cached == false {
						colorMatchCacheLock.Lock()
						colorMatchCache[oldPixel] = beadName
						colorMatchCacheLock.Unlock()
					}

					if len(*htmlFileName) > 0 {
						outputImageBeadNames[pixel.X+pixel.Y*imageBounds.Max.X] = beadName
					}

					matchRgb := beadConfig[beadName]
					setOutputImagePixel(outputImage, pixel, matchRgb)
				}(pixel)
			case <-workDone:
				return
			}
		}
	}()

	go calculateBeadUsage(beadUsageChan)

	startTime := time.Now()
	for y := imageBounds.Min.Y; y < imageBounds.Max.Y; y++ {
		for x := imageBounds.Min.X; x < imageBounds.Max.X; x++ {
			workQueueChan <- image.Point{x, y}
		}
	}

	pixelWaitGroup.Wait() // wait for all pixel to be processed
	workDone <- struct{}{}
	close(workQueueChan)
	close(beadUsageChan)
	<-beadStatsDone

	elapsedTime := time.Since(startTime)
	fmt.Printf("Image processed in %s\n", elapsedTime)

	if len(*htmlFileName) > 0 {
		writeHTMLBeadInstructionFile(*htmlFileName, imageBounds, outputImage, outputImageBeadNames)
	}

	imageWriter, err := os.Create(*outputFileName)
	if err != nil {
		fmt.Printf("Opening image output file failed: %v\n", err)
		os.Exit(1)
	}
	defer imageWriter.Close()

	png.Encode(imageWriter, outputImage)
}

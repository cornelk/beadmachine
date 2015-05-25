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
	kingpin "gopkg.in/alecthomas/kingpin.v1"
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
	inputFileName   = kingpin.Flag("input", "Filename of image to process.").Short('i').Required().String()
	outputFileName  = kingpin.Flag("output", "Output filename for the converted PNG image.").Short('o').PlaceHolder("OUTPUT.png").Required().String()
	htmlFileName    = kingpin.Flag("html", "Output filename for a HTML based bead pattern file.").Short('l').String()
	paletteFileName = kingpin.Flag("palette", "Filename of the bead palette.").Short('p').Default("colors_hama.json").String()
	newWidth        = kingpin.Flag("width", "Resize image to width in pixel.").Short('w').Default("0").Int()
	newHeight       = kingpin.Flag("height", "Resize image to height in pixel.").Short('h').Default("0").Int()
	newWidthBoards  = kingpin.Flag("boardswidth", "Resize image to width in amount of boards.").Short('x').Default("0").Int()
	newHeightBoards = kingpin.Flag("boardsheight", "Resize image to height in amount of boards.").Short('y').Default("0").Int()
	boardDimension  = kingpin.Flag("boarddimension", "Dimension of a board.").Short('d').Default("29").Int()
	beadStyle       = kingpin.Flag("bead", "Make output file look like a beads board.").Short('b').Bool()
	greyScale       = kingpin.Flag("grey", "Convert the image to greyscale.").Short('g').Bool()
	useTranslucent  = kingpin.Flag("translucent", "Include translucent colors for the conversion.").Short('t').Bool()
	useFlourescent  = kingpin.Flag("flourescent", "Include flourescent colors for the conversion.").Short('f').Bool()

	targetIlluminant = &chromath.IlluminantRefD50
	labTransformer   = chromath.NewLabTransformer(targetIlluminant)
	rgbTransformer   = chromath.NewRGBTransformer(&chromath.SpaceSRGB, &chromath.AdaptationBradford, targetIlluminant, &chromath.Scaler8bClamping, 1.0, nil)
	beadFillPixel    = color.RGBA{225, 225, 225, 255} // light grey

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
		//fmt.Println("Loaded bead:", beadName, "RGB:", rgb, "Lab:", lab)
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
		//fmt.Println("Match:", beadName, "with distance:", distance)
	}

	//fmt.Println("Best match:", bestBeadMatch, "with distance:", minDistance)
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
	w.WriteString("<html>\n<body>\n<table>\n")

	for y := outputImageBounds.Min.Y; y < outputImageBounds.Max.Y; y++ {
		w.WriteString("<tr>") // write a line with colored cells
		for x := outputImageBounds.Min.X; x < outputImageBounds.Max.X; x++ {
			pixel := outputImage.RGBAAt(x, y)
			colorstring := fmt.Sprintf("#%02X%02X%02X", pixel.R, pixel.G, pixel.B)
			w.WriteString("<td bgcolor=\"" + colorstring + "\">&nbsp;</td>")
		}
		w.WriteString("</tr>\n")

		w.WriteString("<tr>") // write a line with bead names
		for x := outputImageBounds.Min.X; x < outputImageBounds.Max.X; x++ {
			beadName := outputImageBeadNames[x+y*outputImageBounds.Max.X]
			shortName := strings.Split(beadName, " ")
			w.WriteString("<td>" + shortName[0] + "</td>") // only print first part of name
		}
		w.WriteString("</tr>\n")
	}

	w.WriteString("</table>\n</body>\n</html>\n")
	w.Flush()
	htmlFile.Close()

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
	fmt.Println("Input image width:", imageBounds.Dx(), "height:", imageBounds.Dy())

	if *greyScale { // better looking results when doing greyscaling before resizing
		inputImage = imaging.Grayscale(inputImage)
	}

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

	outputImageBounds := imageBounds
	fmt.Println("Bead boards width:", calculateBeadBoardsNeeded(outputImageBounds.Dx()), "height:", calculateBeadBoardsNeeded(outputImageBounds.Dy()))
	if *beadStyle { // each pixel will be a bead of 8x8 pixel
		outputImageBounds.Max.X *= 8
		outputImageBounds.Max.Y *= 8
	}
	if resized || *beadStyle {
		fmt.Println("Output image pixel width:", imageBounds.Dx(), "height:", imageBounds.Dy())
	}
	fmt.Printf("Beads width: %v cm, height: %v cm\n", float64(imageBounds.Dx())*0.5, float64(imageBounds.Dy())*0.5)
	outputImage := image.NewRGBA(outputImageBounds)

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
		writeHTMLBeadInstructionFile(*htmlFileName, outputImageBounds, outputImage, outputImageBeadNames)
	}

	imageWriter, err := os.Create(*outputFileName)
	if err != nil {
		fmt.Printf("Opening image output file failed: %v\n", err)
		os.Exit(1)
	}
	defer imageWriter.Close()

	png.Encode(imageWriter, outputImage)
}

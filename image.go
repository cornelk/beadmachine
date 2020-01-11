package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"runtime"
	"sync"

	"github.com/disintegration/imaging"
)

// readImageFile reads and decodes the given image file
func readImageFile(FileName string) image.Image {
	imageReader, err := os.Open(*inputFileName)
	if err != nil {
		fmt.Printf("Opening input image file failed: %v\n", err)
		os.Exit(1)
	}
	defer imageReader.Close()

	inputImage, _, err := image.Decode(imageReader)
	if err != nil {
		fmt.Printf("Decoding input image file failed: %v\n", err)
		os.Exit(1)
	}

	return inputImage
}

// processImage matches all pixel of the image to a matching bead
func processImage(imageBounds image.Rectangle, inputImage image.Image, outputImage *image.RGBA, paletteFileNameToLoad string) {
	beadConfig, beadLab := LoadPalette(paletteFileNameToLoad)

	pixelCount := imageBounds.Dx() * imageBounds.Dy()
	beadUsageChan := make(chan string, pixelCount)
	workQueueChan := make(chan image.Point, runtime.NumCPU()*2)
	workDone := make(chan struct{})

	var outputImageBeadNames []string // TODO use pointer to bead config instead of string
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
					beadName := FindSimilarColor(beadLab, oldPixel)
					beadUsageChan <- beadName

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

	if len(*htmlFileName) > 0 {
		writeHTMLBeadInstructionFile(*htmlFileName, imageBounds, outputImage, outputImageBeadNames)
	}
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

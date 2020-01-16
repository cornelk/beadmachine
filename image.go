package main

import (
	"image"
	"image/color"
	"os"
	"runtime"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/pkg/errors"
)

// readImageFile reads and decodes the given image file
func readImageFile(FileName string) (image.Image, error) {
	imageReader, err := os.Open(FileName)
	if err != nil {
		return nil, errors.Wrap(err, "opening image file")
	}
	defer imageReader.Close()

	inputImage, _, err := image.Decode(imageReader)
	if err != nil {
		return nil, errors.Wrap(err, "decoding image file")
	}

	return inputImage, nil
}

// processImage matches all pixel of the image to a matching bead
func (m *beadMachine) processImage(imageBounds image.Rectangle, inputImage image.Image, outputImage *image.RGBA, paletteFileNameToLoad string) error {
	beadConfig, beadLab, err := m.LoadPalette(paletteFileNameToLoad)
	if err != nil {
		return err
	}

	pixelCount := imageBounds.Dx() * imageBounds.Dy()
	beadUsageChan := make(chan string, pixelCount)
	workQueueChan := make(chan image.Point, runtime.NumCPU()*2)
	workDone := make(chan struct{})

	var outputImageBeadNames []string // TODO use pointer to bead config instead of string
	if m.htmlFileName != "" {
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
					beadName := m.FindSimilarColor(beadLab, oldPixel)
					beadUsageChan <- beadName

					if m.htmlFileName != "" {
						outputImageBeadNames[pixel.X+pixel.Y*imageBounds.Max.X] = beadName
					}

					matchRgb := beadConfig[beadName]
					m.setOutputImagePixel(outputImage, pixel, matchRgb)
				}(pixel)
			case <-workDone:
				return
			}
		}
	}()

	go m.calculateBeadUsage(beadUsageChan)

	for y := imageBounds.Min.Y; y < imageBounds.Max.Y; y++ {
		for x := imageBounds.Min.X; x < imageBounds.Max.X; x++ {
			workQueueChan <- image.Point{x, y}
		}
	}

	pixelWaitGroup.Wait() // wait for all pixel to be processed
	workDone <- struct{}{}
	close(workQueueChan)
	close(beadUsageChan)
	<-m.beadStatsDone

	if m.htmlFileName != "" {
		return m.writeHTMLBeadInstructionFile(m.htmlFileName, imageBounds, outputImage, outputImageBeadNames)
	}
	return nil
}

// applyfilters will apply all filters that were enabled to the input image
func (m *beadMachine) applyFilters(inputImage image.Image) image.Image {
	filteredImage := inputImage

	if m.greyScale {
		filteredImage = imaging.Grayscale(filteredImage)
	}
	if m.blur != 0.0 {
		filteredImage = imaging.Blur(filteredImage, m.blur)
	}
	if m.sharpen != 0.0 {
		filteredImage = imaging.Sharpen(filteredImage, m.sharpen)
	}
	if m.gamma != 0.0 {
		filteredImage = imaging.AdjustGamma(filteredImage, m.gamma)
	}
	if m.contrast != 0.0 {
		filteredImage = imaging.AdjustContrast(filteredImage, m.contrast)
	}
	if m.brightness != 0.0 {
		filteredImage = imaging.AdjustBrightness(filteredImage, m.brightness)
	}

	return filteredImage
}

// setOutputImagePixel sets a pixel in the output image or draws a bead in beadStyle mode
func (m *beadMachine) setOutputImagePixel(outputImage *image.RGBA, coordinates image.Point, bead BeadConfig) {
	rgbaMatch := color.RGBA{bead.R, bead.G, bead.B, 255} // A 255 = no transparency
	if m.beadStyle {
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				if (x%7 == 0 && y%7 == 0) || (x > 2 && x < 5 && y > 2 && y < 5) { // all corner pixel + 2x2 in center
					outputImage.SetRGBA((coordinates.X*8)+x, (coordinates.Y*8)+y, m.beadFillPixel)
				} else {
					outputImage.SetRGBA((coordinates.X*8)+x, (coordinates.Y*8)+y, rgbaMatch)
				}
			}
		}
	} else {
		outputImage.SetRGBA(coordinates.X, coordinates.Y, rgbaMatch)
	}
}

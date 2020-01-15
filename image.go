package main

import (
	"image"
	"image/color"
	"os"
	"runtime"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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
func processImage(cmd *cobra.Command, logger *zap.Logger, imageBounds image.Rectangle, inputImage image.Image, outputImage *image.RGBA, paletteFileNameToLoad string) error {
	beadConfig, beadLab, err := LoadPalette(cmd, logger, paletteFileNameToLoad)
	if err != nil {
		return err
	}

	pixelCount := imageBounds.Dx() * imageBounds.Dy()
	beadUsageChan := make(chan string, pixelCount)
	workQueueChan := make(chan image.Point, runtime.NumCPU()*2)
	workDone := make(chan struct{})

	var outputImageBeadNames []string // TODO use pointer to bead config instead of string
	htmlFileName, _ := cmd.Flags().GetString("html")
	if htmlFileName != "" {
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
					beadName := FindSimilarColor(logger, beadLab, oldPixel)
					beadUsageChan <- beadName

					if htmlFileName != "" {
						outputImageBeadNames[pixel.X+pixel.Y*imageBounds.Max.X] = beadName
					}

					matchRgb := beadConfig[beadName]
					setOutputImagePixel(cmd, outputImage, pixel, matchRgb)
				}(pixel)
			case <-workDone:
				return
			}
		}
	}()

	go calculateBeadUsage(logger, beadUsageChan)

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

	if htmlFileName != "" {
		return writeHTMLBeadInstructionFile(cmd, htmlFileName, imageBounds, outputImage, outputImageBeadNames)
	}
	return nil
}

// applyfilters will apply all filters that were enabled to the input image
func applyFilters(cmd *cobra.Command, inputImage image.Image) image.Image {
	filteredImage := inputImage

	greyScale, _ := cmd.Flags().GetBool("grey")
	if greyScale {
		filteredImage = imaging.Grayscale(filteredImage)
	}
	filterBlur, _ := cmd.Flags().GetFloat64("blur")
	if filterBlur != 0.0 {
		filteredImage = imaging.Blur(filteredImage, filterBlur)
	}
	filterSharpen, _ := cmd.Flags().GetFloat64("sharpen")
	if filterSharpen != 0.0 {
		filteredImage = imaging.Sharpen(filteredImage, filterSharpen)
	}
	filterGamma, _ := cmd.Flags().GetFloat64("gamma")
	if filterGamma != 0.0 {
		filteredImage = imaging.AdjustGamma(filteredImage, filterGamma)
	}
	filterContrast, _ := cmd.Flags().GetFloat64("contrast")
	if filterContrast != 0.0 {
		filteredImage = imaging.AdjustContrast(filteredImage, filterContrast)
	}
	filterBrightness, _ := cmd.Flags().GetFloat64("brightness")
	if filterBrightness != 0.0 {
		filteredImage = imaging.AdjustBrightness(filteredImage, filterBrightness)
	}

	return filteredImage
}

// setOutputImagePixel sets a pixel in the output image or draws a bead in beadStyle mode
func setOutputImagePixel(cmd *cobra.Command, outputImage *image.RGBA, coordinates image.Point, bead BeadConfig) {
	rgbaMatch := color.RGBA{bead.R, bead.G, bead.B, 255} // A 255 = no transparency
	beadStyle, _ := cmd.Flags().GetBool("beadstyle")
	if beadStyle {
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

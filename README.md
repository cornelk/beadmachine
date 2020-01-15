# beadmachine

A bead pattern creator. Convert any imagine into a suitable color palette pixel by pixel in order to be able to create a matching, beadable pattern.
It also shows you a statistic about the used beads.

## Features

- Cross platform
- Uses all available CPU cores to process the image
- Supports gif/jpg/png as input file formats
- Can output a HTML file with detailed info on which bead to use for each pixel
- Color matching based on [CIEDE2000](http://en.wikipedia.org/wiki/Color_difference#CIEDE2000 "")
- Included bead palettes: [Hama](http://www.hama.dk "")
- Optional image resizing
- Image filters to preprocess the input image

## Installation

You need to have Golang installed, otherwise follow the guide at [https://golang.org/doc/install](https://golang.org/doc/install).

```
go get github.com/cornelk/beadmachine
```

## Command-line options:
```
Bead pattern creator

Usage:
  beadmachine file.jpg [flags]

Flags:
  -b, --beadstyle            make output file look like a beads board
      --blur float           apply blur filter (0.0 - 10.0)
  -d, --boarddimension int   dimension of a board (default 20)
  -y, --boardsheight int     resize image to height in amount of boards
  -x, --boardswidth int      resize image to width in amount of boards
      --brightness float     apply brightness adjustment (-100 - 100)
      --contrast float       apply contrast adjustment (-100 - 100)
  -f, --flourescent          include flourescent colors for the conversion
      --gamma float          apply gamma correction (0.0 - 10.0)
  -g, --grey                 convert the image to greyscale
  -e, --height int           resize image to height in pixel
  -h, --help                 help for beadmachine
  -l, --html string          output filename for a HTML based bead pattern file
  -i, --input string         image to process
  -n, --nocolormatching      skip the bead color matching
  -o, --output string        output filename for the converted PNG image
  -p, --palette string       filename of the bead palette (default "colors_hama.json")
      --sharpen float        apply sharpen filter (0.0 - 10.0)
  -t, --translucent          include translucent colors for the conversion
  -v, --verbose              verbose output
  -w, --width int            resize image to width in pixel
```

## Example Usage
To convert the sample yoshi image to Hama bead colors:

```bash
./beadmachine -i examples/yoshi_thinking_in.png -o out.png -l pattern.html
```

<img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/yoshi_thinking_in.png" alt="Yoshi thinking in" height="96" width="84"/> -> <img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/yoshi_thinking_out.png" alt="Yoshi thinking out" height="96" width="84"/>

And will print out a statistic:

```bash
Input image width: 28, height: 32
Bead boards width: 1, height: 1
Beads width: 14 cm, height: 16 cm
Bead colors used: 9
Beads used 'H10 Green': 30
Beads used 'H47 Pastel Green': 72
Beads used 'H1 White': 525
Beads used 'H37 Neon green': 38
Beads used 'H38 Neon orange': 18
Beads used 'H4 Orange': 10
Beads used 'H35 Neon red': 13
Beads used 'H27 Beige': 11
Beads used 'H18 Black': 179
Image processed in 6.0004ms
```

The output of the HTML pattern file will look like this:

<img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/yoshi_thinking_htmlpattern.png" alt="Yoshi HTML pattern"/>

To convert the sample Mona Lisa image to Hama bead colors, resize to a width of 58 pixel and create a bead style output:

```bash
./beadmachine -i examples/mona_lisa_in.jpg -o out.png -w 58 -b --blur 2.75 --contrast 10
```

<img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/mona_lisa_in.jpg" alt="Mona Lisa in" height="461" width="310"/> -> <img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/mona_lisa_out.png" alt="Mona Lisa out" height="461" width="310"/>

And will print out a statistic:
```bash
Input image width: 722, height: 1074
Bead boards width: 2, height: 3
Beads width: 29 cm, height: 43 cm
Output image pixel width: 58, height: 86
Bead colors used: 22
...
```

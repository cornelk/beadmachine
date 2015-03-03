# beadmachine
beadmachine is a bead pattern creator. Convert any imagine into a suitable color palette pixel by pixel in order to be able to create a matching, beadable pattern. It also shows you a statistic about the used beads.

### Features
- Cross platform
- Uses all available cores to process the image
- Supports gif/jpg/png as input file formats
- Color matching based on [CIEDE2000](http://en.wikipedia.org/wiki/Color_difference#CIEDE2000 "")
- Included bead palettes: [Hama](http://www.hama.dk "")

### Command-line options:
<dl>
<dt>-i=FILENAME</dt>
  <dd>Filename of image to process.</dd>
<dt>-o=FILENAME.png</dt>
  <dd>Output filename for the converted PNG image.</dd>
<dt>-p=colors_hama.json</dt>
  <dd>Filename of the bead palette.</dd>
</dl>

### Example Usage
To convert the sample yoshi image to Hama bead colors:

```bash
./beadmachine -i examples/yoshi_thinking_in.png -o out.png
```

<img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/yoshi_thinking_in.png" alt="Yoshi thinking in" height="64" width="58"/> converts to <img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/yoshi_thinking_out.png" alt="Yoshi thinking in" height="64" width="58"/>

And will print out a statistic:
```
Image processed in 8.0004ms
Bead colors used: 9
Beads used for color 'H42 Flourescent green': 38
Beads used for color 'H10 Green': 30
Beads used for color 'H47 Pastel Green': 72
Beads used for color 'H4 Orange': 10
Beads used for color 'H35 Neon red': 13
Beads used for color 'H1 White': 525
Beads used for color 'H18 Black': 179
Beads used for color 'H38 Neon orange': 18
Beads used for color 'H27 Beige': 11
```

### Installation
You need to have a [Golang](http://golang.org/doc/install "") environment set up first. Download beadmachine:

```bash
go get github.com/CornelK/beadmachine
```

Change to the directory inside your GOPATH and build the application:

```bash
cd $GOPATH/src/github.com/CornelK/beatmachine
go build
```

### Todo
- Input image resizing
- Export a text based beadable pattern
- Create a bead like output image
- Perler palette
- Support for giving bead stocks as input
- GUI
